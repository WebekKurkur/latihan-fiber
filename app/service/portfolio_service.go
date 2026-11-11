package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"io"
	"strings"
	"time"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

type PortfolioService struct {
	students   repository.StudentRepository
	portfolios repository.PortfolioRepository
	perms      *helper.PermissionSet
}

func NewPortfolioService(students repository.StudentRepository, portfolios repository.PortfolioRepository, perms *helper.PermissionSet) *PortfolioService {
	return &PortfolioService{students: students, portfolios: portfolios, perms: perms}
}

func (s *PortfolioService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()
	studentID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id student harus berupa angka positif")
	}
	if err := s.authorizeAndCheckStudent(ctx, c, studentID); err != nil {
		return err
	}
	portfolio, err := s.portfolios.FindByStudentID(ctx, studentID)
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Ok(c, "portofolio berhasil diambil", portfolio)
}

func (s *PortfolioService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()
	studentID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id student harus berupa angka positif")
	}
	if err := s.authorizeAndCheckStudent(ctx, c, studentID); err != nil {
		return err
	}
	var req model.ReplacePortfolioRequest
	if err := decodePortfolioJSON(c, &req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Headline, req.Bio = strings.TrimSpace(req.Headline), strings.TrimSpace(req.Bio)
	for i := range req.Projects {
		req.Projects[i].Title = strings.TrimSpace(req.Projects[i].Title)
		req.Projects[i].Technologies = NormalizeTechnologies(req.Projects[i].Technologies)
	}
	for i := range req.Certificates {
		req.Certificates[i].Name = strings.TrimSpace(req.Certificates[i].Name)
		req.Certificates[i].Issuer = strings.TrimSpace(req.Certificates[i].Issuer)
	}
	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}
	projects := make([]model.PortfolioProject, 0, len(req.Projects))
	for _, p := range req.Projects {
		projects = append(projects, model.PortfolioProject{Title: p.Title, Technologies: p.Technologies, Link: p.Link, Year: p.Year})
	}
	certificates := make([]model.PortfolioCertificate, 0, len(req.Certificates))
	for _, cert := range req.Certificates {
		certificates = append(certificates, model.PortfolioCertificate{Name: cert.Name, Issuer: cert.Issuer, IssuedAt: cert.IssuedAt.UTC().Truncate(time.Millisecond)})
	}
	portfolio, err := s.portfolios.Replace(ctx, model.StudentPortfolio{StudentID: studentID, Headline: req.Headline, Bio: req.Bio, Projects: projects, Certificates: certificates})
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Ok(c, "portofolio berhasil disimpan", portfolio)
}

func (s *PortfolioService) AddProject(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()
	studentID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id student harus berupa angka positif")
	}
	if err := s.authorizeAndCheckStudent(ctx, c, studentID); err != nil {
		return err
	}
	var req model.PortfolioProjectInput
	if err := decodePortfolioJSON(c, &req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Technologies = NormalizeTechnologies(req.Technologies)
	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}
	portfolio, err := s.portfolios.AddProject(ctx, studentID, model.PortfolioProject{Title: req.Title, Technologies: req.Technologies, Link: req.Link, Year: req.Year})
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Ok(c, "proyek berhasil ditambahkan", portfolio)
}

func (s *PortfolioService) DeleteProject(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()
	studentID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("id student harus berupa angka positif")
	}
	if err := s.authorizeAndCheckStudent(ctx, c, studentID); err != nil {
		return err
	}
	projectID := c.Params("projectId")
	if err := s.portfolios.DeleteProject(ctx, studentID, projectID); err != nil {
		return translatePortfolioError(err)
	}
	return helper.NoContent(c)
}

func (s *PortfolioService) TopTechnologies(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()
	stats, err := s.portfolios.TopTechnologies(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Ok(c, "statistik teknologi berhasil diambil", stats)
}

// List menampilkan seluruh portofolio dengan cursor pagination.
//
// Endpoint ini memakai keyset pagination, bukan nomor halaman: portofolio
// yang baru diperbarui dapat muncul di awal daftar tanpa menggeser isi
// halaman yang sudah dibaca klien.
//
// Bentuk response memakai model.PortfolioPage — bukan fiber.Map — agar
// swag dapat mendokumentasikan isinya.
func (s *PortfolioService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return helper.BadRequest("cursor tidak sah")
	}

	rows, err := s.portfolios.FindAllAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}

	// Baris tambahan hasil limit+1 dipotong di sini. Ia hanya penanda bahwa
	// masih ada halaman berikutnya, bukan bagian dari halaman ini.
	hasMore := len(rows) > q.Limit
	if hasMore {
		rows = rows[:q.Limit]
	}

	meta := model.PortfolioPageMeta{Limit: q.Limit, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta.NextCursor = helper.EncodeCursor(last.UpdatedAt, last.StudentID)
	}

	return c.Status(fiber.StatusOK).JSON(model.PortfolioPage{
		Success: true,
		Message: "daftar portofolio berhasil diambil",
		Data:    rows,
		Meta:    meta,
	})
}

func (s *PortfolioService) authorizeAndCheckStudent(ctx context.Context, c *fiber.Ctx, studentID int) error {
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	if !CanAccessStudent(current, &studentID, s.perms, "student:read:any") {
		return helper.Forbidden("tidak berhak mengakses portofolio student lain")
	}
	_, err := s.students.FindByID(ctx, studentID)
	if err != nil {
		return translatePortfolioError(err)
	}
	return nil
}

// translatePortfolioError menerjemahkan error dari repository menjadi AppError
// dengan status HTTP dan kode yang konsisten untuk seluruh endpoint portofolio.
func translatePortfolioError(err error) error {
	switch {
	case errors.Is(err, repository.ErrPortfolioInvalidProjectID):
		return helper.BadRequest("id proyek tidak valid")
	case errors.Is(err, repository.ErrPortfolioNotFound):
		return helper.NotFound("portofolio tidak ditemukan")
	case errors.Is(err, repository.ErrPortfolioProjectLimit):
		return helper.Conflict("jumlah proyek sudah mencapai batas 20")
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("student tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("nim sudah dipakai")
	case errors.Is(err, repository.ErrEmailDuplicate):
		return helper.Conflict("email sudah dipakai")
	default:
		return helper.Internal(err)
	}
}

func decodePortfolioJSON(c *fiber.Ctx, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("body berisi data tambahan")
		}
		return err
	}
	return nil
}
