package main

import "fmt"

type Mhs struct {
	ID       int
	Name     string
	Grade    float64
	isActive bool
}

//value receiver
func (m Mhs) getInfo() string {
	return fmt.Sprintf(
		"ID: %d, Name: %s, Grade: %.2f, Active: %t",
		m.ID,
		m.Name,
		m.Grade,
		m.isActive,
	)
}

//pointer receiver
func (m *Mhs) updateGrade(Grade float64) {
	m.Grade = Grade
}

//pointer receiver
func (m *Mhs) Activate() {
	m.isActive = true
}

//pointer receiver
func (m *Mhs) Deactivate() {
	m.isActive = false
}

func main() {
	mhs := Mhs{
		ID:       1,
		Name:     "Raka",
		Grade:    85,
		isActive: true,
	}

	fmt.Println("data awal mhs:")
	fmt.Println(mhs.getInfo())

	//update Grade
	mhs.updateGrade(95)
	fmt.Println("Setelah update grade:")
	fmt.Println(mhs.getInfo())

	//update status keaktifan
	mhs.Deactivate()
	fmt.Println("Setelah deactivate:")
	fmt.Println(mhs.getInfo())

	mhs.Activate()
	fmt.Println("Setelah update activate:")
	fmt.Println(mhs.getInfo())
}
