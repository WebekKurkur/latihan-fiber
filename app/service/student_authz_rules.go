package service

import (
	"strings"

	"latihan-fiber/app/model"
	"latihan-fiber/helper"
)

// CanAccessStudent memutuskan apakah seseorang boleh menyentuh data student.
//
// Dua jalur yang diizinkan:
//  1. Kepemilikan (ownership) — data itu miliknya sendiri.
//  2. Permission — role-nya memang berhak atas data siapa pun.
//
// Urutannya disengaja: pemeriksaan kepemilikan didahulukan karena paling
// murah dan paling sering benar. Bila keduanya gagal, jawabannya false.
//
// ownerID boleh nil untuk data lama yang belum memiliki pemilik; dalam
// keadaan itu jalur ownership otomatis tidak terpenuhi dan keputusan
// sepenuhnya jatuh kepada permission.
func CanAccessStudent(
	current model.AuthStudents,
	targetId *int,
	perms *helper.PermissionSet,
	anyPermission string,
) bool {
	// fmt.Println("CanAccessStudent: current", current, "targetId", targetId, "anyPermission", anyPermission)
	if targetId != nil && current.StudentID == *targetId {
		return true
	}
	return perms.Can(current.Role, anyPermission)
}

// ValidateAssignRole memeriksa permintaan pergantian role.
//
// Perhatikan aturan terakhir: seseorang tidak boleh mengubah role dirinya
// sendiri. Tanpa aturan itu, satu-satunya admin dapat menurunkan dirinya
// sendiri menjadi student biasa dan sistem kehilangan admin selamanya.
//
// current adalah identitas student yang sedang login; targetID adalah id
// student yang akan diubahkan rolenya. Pada entity students, keduanya
// berasal dari tabel yang sama, sehingga perbandingan langsung dimungkinkan.
func ValidateAssignRole(
	current model.AuthStudents,
	targetID int,
	req model.AssignRoleRequest,
	perms *helper.PermissionSet,
) map[string]string {
	errs := map[string]string{}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		errs["role"] = "wajib diisi"
		return errs
	}

	if !perms.IsKnownRole(role) {
		errs["role"] = "role tidak dikenal, pilih salah satu dari: " +
			strings.Join(perms.KnownRoles(), ", ")
	}

	if current.StudentID == targetID {
		errs["role"] = "tidak boleh mengubah role diri sendiri"
	}

	return errs
}
