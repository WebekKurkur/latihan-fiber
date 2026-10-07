package model

type Student struct {
	ID       int      `json:"id"`
	UserID   int      `json:"user_id"`
	NIM      string   `json:"nim"`
	Nama     string   `json:"nama"`
	Prodi    string   `json:"prodi"`
	Angkatan int      `json:"angkatan"`
	IPK      *float64 `json:"ipk_terakhir"`
}
type StudentDetail struct {
	Student
	MataKuliah []Enrollment `json:"mata_kuliah"`
	TotalSKS   int          `json:"total_sks"`
	BatasSKS   int          `json:"batas_sks"`
}
