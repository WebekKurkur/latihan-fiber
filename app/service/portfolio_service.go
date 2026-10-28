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
	if errors.Is(err, repository.ErrPortfolioNotFound) {
		return helper.NotFound("portofolio tidak ditemukan")
	}
	if err != nil {
		return helper.Internal(err)
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
		for j := range req.Projects[i].Technologies {
			req.Projects[i].Technologies[j] = strings.TrimSpace(req.Projects[i].Technologies[j])
		}
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
		return helper.Internal(err)
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
	for i := range req.Technologies {
		req.Technologies[i] = strings.TrimSpace(req.Technologies[i])
	}
	if fields := helper.ValidateStruct(req); fields != nil {
		return helper.Validation(fields)
	}
	portfolio, err := s.portfolios.AddProject(ctx, studentID, model.PortfolioProject{Title: req.Title, Technologies: req.Technologies, Link: req.Link, Year: req.Year})
	switch {
	case errors.Is(err, repository.ErrPortfolioProjectLimit):
		return helper.Conflict("jumlah proyek sudah mencapai batas 20")
	case errors.Is(err, repository.ErrPortfolioNotFound):
		return helper.NotFound("portofolio tidak ditemukan")
	case err != nil:
		return helper.Internal(err)
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
		if errors.Is(err, repository.ErrPortfolioInvalidProjectID) {
			return helper.BadRequest("id proyek tidak valid")
		}
		if errors.Is(err, repository.ErrPortfolioNotFound) {
			return helper.NotFound("portofolio atau proyek tidak ditemukan")
		}
		return helper.Internal(err)
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

func (s *PortfolioService) authorizeAndCheckStudent(ctx context.Context, c *fiber.Ctx, studentID int) error {
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	if !CanAccessStudent(current, &studentID, s.perms, "student:read:any") {
		return helper.Forbidden("tidak berhak mengakses portofolio student lain")
	}
	_, err := s.students.FindByID(ctx, studentID)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("student tidak ditemukan")
	}
	if err != nil {
		return helper.Internal(err)
	}
	return nil
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
