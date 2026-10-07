package model

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type CreateStudentRequest struct {
	NIM      string   `json:"nim" validate:"required,nim"`
	Nama     string   `json:"nama" validate:"required"`
	Email    string   `json:"email" validate:"required,email"`
	Prodi    string   `json:"prodi" validate:"required"`
	Angkatan int      `json:"angkatan" validate:"required,angkatan"`
	IPK      *float64 `json:"ipk_terakhir" validate:"omitempty,gte=0,lte=4"`
}

type UpdateStudentRequest struct {
	Nama     string   `json:"nama" validate:"required"`
	Prodi    string   `json:"prodi" validate:"required"`
	Angkatan int      `json:"angkatan" validate:"required,angkatan"`
	IPK      *float64 `json:"ipk_terakhir" validate:"omitempty,gte=0,lte=4"`
}

type CreateEnrollmentRequest struct {
	CourseID      int    `json:"course_id" validate:"required,gt=0"`
	TahunAkademik string `json:"tahun_akademik" validate:"required,tahunakademik"`
}

type PageQuery struct {
	Page     int
	PerPage  int
	Prodi    string
	Angkatan int
	Search   string
	Sort     string
}

type CourseQuery struct {
	Semester  int
	Search    string
	Available bool
}
