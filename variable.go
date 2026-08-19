package main

import "fmt"

func main() {
	var nama string = "abdurrahman adil ambagi"
	var umur int = 20
	var gajiku float32 = 9.5
	var boolA bool //langsung terisi sebagai false
	sliceA := []string{"aku", "suka", "nasi goreng"}

	fmt.Println("nama saya", nama)
	fmt.Println("umur saya", umur)
	fmt.Println("gaji saya", gajiku, "aamiin")
	fmt.Println("apa saya salah?", boolA)
	fmt.Println(sliceA)
}
