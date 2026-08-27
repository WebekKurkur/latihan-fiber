package main

import "fmt"

func main() {
	//inisialisasi map
	nilaiMhs := make(map[string]int)

	//a. menambah isi map
	nilaiMhs["Raka"] = 85
	nilaiMhs["Habib"] = 92
	nilaiMhs["Irsyad"] = 78
	nilaiMhs["Farel"] = 88
	//print isi map
	fmt.Println(nilaiMhs)

	//b. read map dengan pengecekan pengecekan keberadaan
	target := "Raka"
	if nilai, exists := nilaiMhs[target]; exists {
		fmt.Printf("nilainya %s: %d\n", target, nilai)
	} else {
		fmt.Printf("Data mahasiswa tidak ditemukan.", target)
	}

	//c. menghapus data dari map
	delete(nilaiMhs, "Irsyad")
	fmt.Println("data 'Irsyad' telah dihapus, sisa:", nilaiMhs)

	//d. menelusuri seluruh isi map
	fmt.Println("\nMenelusuri seluruh isi map tersisa:")
	for namaMhs, nilaiAkhir := range nilaiMhs {
		fmt.Printf("- Mahasiswa: %s , Nilai: %d\n", namaMhs, nilaiAkhir)
	}
}
