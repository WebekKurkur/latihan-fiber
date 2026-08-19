package main

import "fmt"

//Menukar nilai dua integer melalui pointer
func swap(a, b *int) {
	temp := *a
	*a = *b
	*b = temp
}

//Menambahkan item baru ke slice
func updateSlice(s *[]string, newItem string) {
	*s = append(*s, newItem)
}

func passByValue(x int) {
	x = 100
}

func passByPointer(x *int) {
	*x = 100
}

func main() {

	//a, function swap
	a := 5
	b := 10

	fmt.Println("a, function swap")
	fmt.Println("sebelum swap:")
	fmt.Println("a:", a)
	fmt.Println("b:", b)

	swap(&a, &b)

	fmt.Println("setelah swap:")
	fmt.Println("a:", a)
	fmt.Println("b:", b)

	//b, function update slice
	fmt.Println("b, function update slice")
	items := []string{"bolu", "bola"}

	fmt.Println("sebelum update slice:")
	fmt.Println(items)

	updateSlice(&items, "bali")

	fmt.Println("setelah update slice:")
	fmt.Println(items)

	//function pass by value
	fmt.Println("perbandingan pass by value dan pass by pointer")
	fmt.Println("pass by value:")

	x := 10

	fmt.Println("sebelum function passByValue:", x)

	passByValue(x)

	fmt.Println("setelah function passByValue:", x)

	//function pass by pointer
	fmt.Println("pass by pointer:")

	y := 10

	fmt.Println("sebelum function passByPointer:", y)

	passByPointer(&y)

	fmt.Println("setelah function passByPointer:", y)
}
