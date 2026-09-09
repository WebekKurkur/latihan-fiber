package service

import (
    "strings"

    "latihan-fiber/app/model"
)

// ValidateCreate memeriksa request POST student.
func ValidateCreate(req model.CreateStudentRequest) map[string]string {
    errs := map[string]string{}

    if strings.TrimSpace(req.Name) == "" {
        errs["name"] = "wajib diisi"
    }
    if strings.TrimSpace(req.NIM) == "" {
        errs["nim"] = "wajib diisi"
    }
    if !validGrade(req.Grade) {
        errs["grade"] = "harus berada pada rentang 0 sampai 100"
    }

    return errs
}

// ValidateReplace memeriksa request PUT student.
func ValidateReplace(req model.ReplaceStudentRequest) map[string]string {
    errs := map[string]string{}

    if strings.TrimSpace(req.Name) == "" {
        errs["name"] = "wajib diisi pada PUT"
    }
    if strings.TrimSpace(req.NIM) == "" {
        errs["nim"] = "wajib diisi pada PUT"
    }
    if !validGrade(req.Grade) {
        errs["grade"] = "harus berada pada rentang 0 sampai 100"
    }

    return errs
}

// ApplyPatch menerapkan field PATCH yang dikirim ke data student saat ini.
func ApplyPatch(
    current model.Student,
    req model.PatchStudentRequest,
) (model.Student, map[string]string) {
    errs := map[string]string{}

    if req.Name != nil {
        if strings.TrimSpace(*req.Name) == "" {
            errs["name"] = "tidak boleh kosong"
        } else {
            current.Name = *req.Name
        }
    }

    if req.NIM != nil {
        if strings.TrimSpace(*req.NIM) == "" {
            errs["nim"] = "tidak boleh kosong"
        } else {
            current.NIM = *req.NIM
        }
    }

    if req.Grade != nil {
        if !validGrade(*req.Grade) {
            errs["grade"] = "harus berada pada rentang 0 sampai 100"
        } else {
            current.Grade = *req.Grade
        }
    }

    if req.IsActive != nil {
        current.IsActive = *req.IsActive
    }

    return current, errs
}

// IsEmptyPatch memeriksa apakah tidak ada field yang dikirim.
func IsEmptyPatch(req model.PatchStudentRequest) bool {
    return req.Name == nil &&
        req.NIM == nil &&
        req.Grade == nil &&
        req.IsActive == nil
}

// CountTotalPages menghitung jumlah halaman dengan pembulatan ke atas.
func CountTotalPages(total, limit int) int {
    if limit <= 0 {
        return 0
    }

    return (total + limit - 1) / limit
}

func validGrade(grade float64) bool {
    return grade >= 0 && grade <= 100
}