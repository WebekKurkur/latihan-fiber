package model

import "time"

// HealthStatus adalah isi field "data" pada endpoint GET /api/v1/health.
//
// Sebelumnya endpoint ini memakai fiber.Map. Map tidak memiliki nama,
// sehingga swag tidak dapat menjelaskan isinya di dokumentasi.
// Bentuk JSON-nya tetap sama persis: {"timestamp": "..."}.
type HealthStatus struct {
    Timestamp time.Time `json:"timestamp"`
}