package handler

import "database/sql"

// Handler drži zavisnosti koje su potrebne svim handlerima
type Handler struct {
	DB *sql.DB
}

// Novi kreira novi Handler sa datom bazom
func Novi(db *sql.DB) *Handler {
	return &Handler{DB: db}
}
