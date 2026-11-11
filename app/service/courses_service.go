package service

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/helper"
)

// Kesalahan khusus layer service: aturan bisnis yang tidak diketahui
// repository.
var errCreditLimit = errors.New("batas SKS terlampaui")

type CourseService struct {
	courses repository.CourseRepository
	outbox  repository.OutboxRepository
	tx      *repository.TxManager
}

func NewCourseService(
	courses repository.CourseRepository,
	outbox repository.OutboxRepository,
	tx *repository.TxManager,
) *CourseService {
	return &CourseService{courses: courses, outbox: outbox, tx: tx}
}

// ---------- GET /courses ----------

// List godoc
//
// @Summary   Daftar seluruh kelas
// @Tags      courses
// @Produce   json
// @Success   200  {object}  model.WebResponse{data=[]model.Course}
// @Failure   401  {object}  model.ErrorResponse
// @Security  BearerAuth
// @Router    /courses [get]
func (s *CourseService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	courses, err := s.courses.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Ok(c, "daftar kelas berhasil diambil", courses)
}

// ---------- GET /courses/mine ----------

// Mine godoc
//
// @Summary   KRS milik mahasiswa yang sedang login
// @Tags      courses
// @Produce   json
// @Success   200  {object}  model.WebResponse{data=model.MyCourses}
// @Failure   401  {object}  model.ErrorResponse
// @Security  BearerAuth
// @Router    /courses/mine [get]
func (s *CourseService) Mine(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courses, err := s.courses.FindByStudent(ctx, current.StudentID)
	if err != nil {
		return helper.Internal(err)
	}

	total := 0
	for _, course := range courses {
		total += course.Credits
	}
	return helper.Ok(c, "KRS berhasil diambil", model.MyCourses{
		Courses: courses, TotalCredits: total, MaxCredits: MaxCreditsPerSemester,
	})

}

// ---------- GET /courses/:id ----------

// Get godoc
//
// @Summary   Detail satu kelas
// @Tags      courses
// @Produce   json
// @Param     id   path      int  true  "ID kelas"
// @Success   200  {object}  model.WebResponse{data=model.Course}
// @Failure   400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure   404  {object}  model.ErrorResponse
// @Security  BearerAuth
// @Router    /courses/{id} [get]
func (s *CourseService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	course, err := s.courses.FindByID(ctx, id)
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Ok(c, "kelas ditemukan", course)
}

// ---------- POST /courses/:id/enroll ----------

// Enroll godoc
//
// @Summary      Mendaftarkan ke sebuah kelas
// @Description  Mengambil satu kursi dan mencatat event enrollment.created ke outbox dalam SATU transaction.
// @Description  Kode error 409: COURSE_FULL, ALREADY_ENROLLED, CREDIT_LIMIT_EXCEEDED.
// @Tags         courses
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "ID kelas"
// @Success      201  {object}  model.WebResponse{data=model.Course}
// @Header       201  {string}  Location  "URL kelas yang baru didaftari"
// @Failure      401  {object}  model.ErrorResponse
// @Failure      404  {object}  model.ErrorResponse
// @Failure      409  {object}  model.ErrorResponse  "Kuota penuh, sudah terdaftar, atau batas SKS terlampaui"
// @Security     BearerAuth
// @Router       /courses/{id}/enroll [post]
func (s *CourseService) Enroll(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var enrolled model.Course

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		// Seluruh query di bawah ini WAJIB memakai repo, bukan s.courses.
		// s.courses terikat pada pool: query lewat s.courses berjalan di
		// luar transaction dan tidak ikut dibatalkan bila langkah
		// berikutnya gagal.
		repo := s.courses.WithTx(tx)

		// 1. Kunci KRS milik mahasiswa ini. Request lain dari mahasiswa
		//    yang SAMA harus menunggu sampai transaction ini selesai.
		if err := repo.LockStudent(ctx, current.StudentID); err != nil {
			return err
		}

		// 2. Periksa batas SKS. Aman, karena tidak ada request lain milik
		//    mahasiswa ini yang dapat menambah SKS di antara pemeriksaan
		//    dan penyimpanan.
		course, err := repo.FindByID(ctx, courseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.StudentID)
		if err != nil {
			return err
		}
		if !WithinCreditLimit(taken, course.Credits) {
			return errCreditLimit
		}

		// 3. Ambil satu kursi secara atomik.
		enrolled, err = repo.IncrementEnrolled(ctx, courseID)
		if err != nil {
			return err
		}

		// 4. Catat pendaftarannya. Bila ternyata sudah terdaftar, error
		//    di sini membatalkan juga penambahan kursi pada langkah 3.
		if err := repo.InsertEnrollment(ctx, courseID, current.StudentID); err != nil {
			return err
		}

		// 5. Catat event KRS ke outbox PADA TRANSACTION YANG SAMA.
		//    Bila pencatatan gagal, seluruh perubahan KRS ikut dibatalkan:
		//    tidak boleh ada KRS yang berubah tanpa notifikasi. Karena itu
		//    outbox WAJIB lewat WithTx(tx), bukan repo yang terikat pool.
		return s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentCreated, model.EnrollmentEvent{
			StudentID:  current.StudentID,
			CourseID:   enrolled.ID,
			CourseCode: enrolled.Code,
			CourseName: enrolled.Name,
		})
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Created(c, "berhasil mendaftar kelas", enrolled,
		"/api/v1/courses/"+strconv.Itoa(courseID))
}

// ---------- DELETE /courses/:id/enroll ----------

// Drop godoc
//
// @Summary   Keluar dari sebuah kelas
// @Tags      courses
// @Param     id   path  int  true  "ID kelas"
// @Success   204  "Berhasil keluar; tanpa body"
// @Failure   401  {object}  model.ErrorResponse
// @Failure   409  {object}  model.ErrorResponse  "NOT_ENROLLED"
// @Security  BearerAuth
// @Router    /courses/{id}/enroll [delete]
func (s *CourseService) Drop(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		if err := repo.LockStudent(ctx, current.StudentID); err != nil {
			return err
		}
		// Ambil data kelas lebih dulu: nama dan kode kelas disalin ke
		// event, supaya notifikasi memakai nama yang dilihat mahasiswa
		// saat keluar dari kelas tersebut.
		course, err := repo.FindByID(ctx, courseID)
		if err != nil {
			return err
		}
		if err := repo.DeleteEnrollment(ctx, courseID, current.StudentID); err != nil {
			return err
		}
		if _, err := repo.DecrementEnrolled(ctx, courseID); err != nil {
			return err
		}

		// Event yang dicatat adalah event KELUAR. Menulis event
		// "enrollment.created" di sini membuat mahasiswa menerima
		// kabar bahwa ia terdaftar pada kelas yang justru ia tinggalkan.
		return s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentDropped, model.EnrollmentEvent{
			StudentID:  current.StudentID,
			CourseID:   course.ID,
			CourseCode: course.Code,
			CourseName: course.Name,
		})
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.NoContent(c)
}

// ---------- POST /enrollments/swap ----------
//
// Pindah kelas adalah contoh terbaik mengapa transaction dibutuhkan:
// keluar dari kelas lama dan masuk ke kelas baru harus terjadi
// bersama-sama. Bila kelas baru ternyata penuh, mahasiswa harus TETAP
// berada di kelas lamanya — bukan kehilangan keduanya.

// Swap godoc
//
// @Summary      Pindah kelas
// @Description  Keluar dari kelas lama dan masuk ke kelas baru sebagai satu tindakan. Bila kelas baru penuh, mahasiswa tetap di kelas lama.
// @Tags         courses
// @Accept       json
// @Produce      json
// @Param        body  body      model.SwapRequest  true  "Kelas asal dan kelas tujuan"
// @Success      200   {object}  model.WebResponse{data=model.Course}
// @Failure      401   {object}  model.ErrorResponse
// @Failure      409   {object}  model.ErrorResponse  "COURSE_FULL, NOT_ENROLLED, CREDIT_LIMIT_EXCEEDED"
// @Failure      422   {object}  model.ErrorResponse
// @Security     BearerAuth
// @Router       /enrollments/swap [post]
func (s *CourseService) Swap(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.SwapRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if req.FromCourseID == req.ToCourseID {
		return helper.Validation(map[string]string{
			"to_course_id": "tidak boleh sama dengan from_course_id",
		})
	}

	var joined model.Course

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		if err := repo.LockStudent(ctx, current.StudentID); err != nil {
			return err
		}

		// Kunci KEDUA kelas lebih dulu, dalam urutan id menaik. Tanpa
		// langkah ini, mahasiswa A yang pindah 1 -> 2 dan mahasiswa B
		// yang pindah 2 -> 1 pada saat yang sama akan saling menunggu
		// kunci milik satu sama lain: deadlock.
		if err := repo.LockCourses(ctx, req.FromCourseID, req.ToCourseID); err != nil {
			return err
		}

		from, err := repo.FindByID(ctx, req.FromCourseID)
		if err != nil {
			return err
		}
		to, err := repo.FindByID(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.StudentID)
		if err != nil {
			return err
		}
		if CreditsAfterSwap(taken, from.Credits, to.Credits) > MaxCreditsPerSemester {
			return errCreditLimit
		}

		if err := repo.DeleteEnrollment(ctx, req.FromCourseID, current.StudentID); err != nil {
			return err
		}
		if _, err := repo.DecrementEnrolled(ctx, req.FromCourseID); err != nil {
			return err
		}

		joined, err = repo.IncrementEnrolled(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		if err := repo.InsertEnrollment(ctx, req.ToCourseID, current.StudentID); err != nil {
			return err
		}

		// Event pindah kelas memuat KEDUA kelas: kelas yang ditinggalkan
		// dan kelas yang dimasuki, supaya notifikasi bisa menyebut
		// perpindahannya dan bukan sekadar kelas tujuan.
		return s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentSwapped, model.EnrollmentEvent{
			StudentID:      current.StudentID,
			CourseID:       joined.ID,
			CourseCode:     joined.Code,
			CourseName:     joined.Name,
			FromCourseCode: from.Code,
			FromCourseName: from.Name,
		})
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Ok(c, "berhasil pindah kelas", joined)
}

// ---------- PATCH /courses/:id ----------
// Dijaga middleware dengan permission course:manage.

// UpdateQuota godoc
//
// @Summary      Mengubah kuota kelas (admin)
// @Description  Optimistic locking: kirim version yang terakhir dilihat. Membutuhkan permission course:manage.
// @Tags         courses
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "ID kelas"
// @Param        body  body      model.UpdateQuotaRequest  true  "Kuota baru dan version"
// @Success      200   {object}  model.WebResponse{data=model.Course}
// @Failure      403   {object}  model.ErrorResponse
// @Failure      409   {object}  model.ErrorResponse  "VERSION_CONFLICT atau QUOTA_BELOW_ENROLLED"
// @Security     BearerAuth
// @Router       /courses/{id} [patch]
func (s *CourseService) UpdateQuota(c *fiber.Ctx) error {
	ctx, cancel := helper.ReqCtx(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.UpdateQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	// Perubahan kuota dan catatan event-nya HARUS berada dalam satu
	// transaction. Semula UpdateQuota hanya satu pernyataan SQL, sehingga
	// atomisitasnya terjamin oleh PostgreSQL. Begitu outbox ikut
	// ditambahkan, ada DUA pernyataan: UPDATE courses dan INSERT INTO
	// outbox. Dua pernyataan hanya atomik bila dijalankan dalam satu
	// transaction — tanpa itu, ada dua kemungkinan buruk:
	//   - kuota berubah tetapi event gagal ditulis  -> peserta tidak
	//     pernah diberi tahu;
	//   - event tertulis tetapi rollback terjadi    -> peserta diberi tahu
	//     tentang perubahan yang sebenarnya tidak jadi terjadi.
	var updated model.Course

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		// Seluruh query di bawah ini memakai repo hasil WithTx(tx),
		// bukan s.courses yang terikat pada pool. Query lewat s.courses
		// berjalan di luar transaction dan tidak ikut dibatalkan bila
		// langkah berikutnya gagal.
		repo := s.courses.WithTx(tx)

		// 1. Baca kondisi kelas SEBELUM diubah. Nilai ini dipakai untuk
		//    membandingkan kuota lama dan baru, sekaligus untuk mengisi
		//    payload event.
		before, err := repo.FindByID(ctx, id)
		if err != nil {
			return err
		}

		// 2. Ubah kuotanya. Optimistic locking tetap berlaku: version
		//    yang dikirim client harus sama dengan yang ada di database.
		updated, err = repo.UpdateQuota(ctx, id, req.Quota, req.Version)
		if err != nil {
			return err
		}

		// 3. Kuota yang tidak berubah tidak perlu notification. Event
		//    hanya dicatat bila nilai kuota benar-benar berbeda.
		if before.Quota == updated.Quota {
			return nil
		}

		// 4. Ambil seluruh peserta kelas. Daftar ini dibaca di dalam
		//    transaction yang sama dengan perubahan kuota, sehingga
		//    peserta yang baru mendaftar pada saat bersamaan tidak
		//    terlewat dan peserta yang baru keluar tidak ikut diberi tahu.
		studentIDs, err := repo.FindEnrolledStudentIDs(ctx, id)
		if err != nil {
			return err
		}

		// 5. Catat satu event per peserta di outbox. Satu baris outbox
		//    berarti satu task Asynq, sehingga retry sebuah task hanya
		//    menyentuh satu penerima: email peserta sebelumnya tidak
		//    dikirim ulang.
		outbox := s.outbox.WithTx(tx)
		for _, studentID := range studentIDs {
			if err := outbox.Insert(ctx, model.EventQuotaChanged, model.EnrollmentEvent{
				StudentID:  studentID,
				CourseID:   updated.ID,
				CourseCode: updated.Code,
				CourseName: updated.Name,
				OldQuota:   before.Quota,
				NewQuota:   updated.Quota,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Ok(c, "kuota berhasil diubah", updated)
}

// translateCourseError memetakan kegagalan ke response.
//
// Seluruh kegagalan aturan bisnis memakai 409 dengan kode yang berbeda:
// permintaannya sah, tetapi keadaan data saat ini tidak mengizinkan.
// Yang tidak dikenali menjadi 500 — termasuk pelanggaran CHECK constraint,
// karena constraint itu hanya terpicu bila kode kita sendiri yang salah.
func translateCourseError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("kelas tidak ditemukan")
	case errors.Is(err, repository.ErrCourseFull):
		return helper.NewError(fiber.StatusConflict, "COURSE_FULL", "kuota kelas sudah penuh")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.NewError(fiber.StatusConflict, "ALREADY_ENROLLED", "Anda sudah terdaftar di kelas ini")
	case errors.Is(err, repository.ErrNotEnrolled):
		return helper.NewError(fiber.StatusConflict, "NOT_ENROLLED", "Anda tidak terdaftar di kelas tersebut")
	case errors.Is(err, errCreditLimit):
		return helper.NewError(fiber.StatusConflict, "CREDIT_LIMIT_EXCEEDED",
			"batas "+strconv.Itoa(MaxCreditsPerSemester)+" SKS terlampaui")
	case errors.Is(err, repository.ErrVersionConflict):
		return helper.NewError(fiber.StatusConflict, "VERSION_CONFLICT",
			"data kelas sudah diubah pihak lain, muat ulang lalu coba lagi")
	case errors.Is(err, repository.ErrQuotaBelowFilled):
		return helper.NewError(fiber.StatusConflict, "QUOTA_BELOW_ENROLLED",
			"kuota tidak boleh lebih kecil dari jumlah peserta saat ini")
	default:
		return helper.Internal(err)
	}
}
