package service

import "strings"

// NormalizeTechnologies adalah aturan murni: ia tidak menyentuh database
// maupun Fiber, sehingga dapat diuji hanya dengan memanggilnya.
//
// Padanan fungsi ini pada modul adalah NormalizeTags pada layanan ulasan.
// Yang berbeda hanya konteksnya: pada portofolio, daftar teknologi
// equivalen dengan tag — kumpulan label bebas yang ditulis mahasiswa dan
// dibaca orang lain. Karena itu bentuk normalisasinya sama: rapikan
// spasi, buang yang kosong, dan hilangkan duplikat.
//
// Perbedaan yang disengaja: TINDAK KAN tag, karena tag hanya untuk
// pencarian, sedangkan nama teknologi ditampilkan apa adanya di portofolio.
// Huruf besar "PostgreSQL" lebih enak dibaca daripada "postgresql".
// Duplikasi tetap dikenali tanpa memperhatikan huruf besar, memakai kunci
// perbandingan huruf kecil, tetapi TEKS YANG DISIMPAN adalah versi pertama
// yang muncul — urutan kemunculan pertama dipertahankan.
func NormalizeTechnologies(in []string) []string {
	result := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))

	for _, raw := range in {
		// strings.Fields memecah berdasarkan whitespace dan membuang
		// bagian kosong, sekaligus merapikan spasi ganda di tengah.
		tech := strings.Join(strings.Fields(raw), " ")
		if tech == "" {
			continue
		}
		key := strings.ToLower(tech)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, tech)
	}

	return result
}
