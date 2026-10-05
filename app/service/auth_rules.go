package service

// Setelah Langkah 6, seluruh aturan validasi yang dapat dinyatakan sebagai
// tag pindah ke struct model. Pemeriksaan yang TIDAK dapat dinyatakan
// sebagai tag (mis. "patch harus berisi setidaknya satu field", atau
// "tidak boleh mengubah role diri sendiri") tetap tinggal di service
// sebagai fungsi biasa.
//
// Berkas ini tidak lagi memuat ValidateRegister, ValidateLogin, atau
// helper terkait — semuanya sudah digantikan tag validate pada model.
