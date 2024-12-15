package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = ""
	dbname   = "postgres"
)

type Book struct {
	ID                int    `json:"id"`
	Title             string `json:"title"`
	Author            string `json:"author"`
	Quantity          int    `json:"quantity"`
	AvailableQuantity int    `json:"available_quantity"`
}

type User struct {
	UserID    int    `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Checkout struct {
	UserID        int    `json:"user_id"`
	UserFirstName string `json:"user_first_name"`
	UserLastName  string `json:"user_last_name"`
	BookID        int    `json:"book_id"`
	BookTitle     string `json:"book_title"`
}

func connectDB() (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func listBooksHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}

	defer db.Close()

	rows, err := db.Query("SELECT id, author, title, quantity, available_quantity FROM books")
	if err != nil {
		log.Fatal(err)
	}

	var books []Book

	for rows.Next() {
		var book Book
		err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Quantity, &book.AvailableQuantity)
		if err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		books = append(books, book)
	}

	w.Header().Set("Content-Type", "application/json")
	if len(books) == 0 {
		json.NewEncoder(w).Encode([]Book{})
	} else {
		json.NewEncoder(w).Encode(books)
	}
}

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, first_name, last_name FROM users")
	if err != nil {
		http.Error(w, "Error fetching users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var user User
		err := rows.Scan(&user.UserID, &user.FirstName, &user.LastName)
		if err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error with rows", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if len(users) == 0 {
		json.NewEncoder(w).Encode([]User{})
	} else {
		json.NewEncoder(w).Encode(users)
	}
}

func addUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	firstName := r.FormValue("first_name")
	lastName := r.FormValue("last_name")

	if firstName == "" || lastName == "" {
		http.Error(w, "First name and last name are required", http.StatusBadRequest)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO users (first_name, last_name) VALUES ($1, $2)", firstName, lastName)
	if err != nil {
		http.Error(w, "Error adding user to the database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"first_name": firstName,
		"last_name":  lastName,
	})
}

func borrowBookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	bookID := r.FormValue("book_id")

	if userID == "" || bookID == "" {
		http.Error(w, "User ID and Book ID are required", http.StatusBadRequest)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM checkouts WHERE user_id = $1 AND book_id = $2", userID, bookID).Scan(&count)
	if err != nil {
		http.Error(w, "Error checking borrowed books", http.StatusInternalServerError)
		return
	}

	if count > 0 {
		http.Error(w, "You can only borrow one copy of the book", http.StatusBadRequest)
		return
	}

	var availableQuantity int
	err = db.QueryRow("SELECT available_quantity FROM books WHERE id = $1", bookID).Scan(&availableQuantity)
	if err != nil {
		http.Error(w, "Error fetching book availability", http.StatusInternalServerError)
		return
	}

	if availableQuantity <= 0 {
		http.Error(w, "Book is currently not available", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO checkouts (user_id, book_id) VALUES ($1, $2)", userID, bookID)
	if err != nil {
		http.Error(w, "Error borrowing the book", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE books SET available_quantity = available_quantity - 1 WHERE id = $1", bookID)
	if err != nil {
		http.Error(w, "Error updating book availability", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Book borrowed successfully",
		"user_id": userID,
		"book_id": bookID,
	})
}

func returnBookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	bookID := r.FormValue("book_id")

	if userID == "" || bookID == "" {
		http.Error(w, "User ID and Book ID are required", http.StatusBadRequest)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM checkouts WHERE user_id = $1 AND book_id = $2", userID, bookID).Scan(&count)
	if err != nil {
		http.Error(w, "Error checking borrowed books", http.StatusInternalServerError)
		return
	}

	if count == 0 {
		http.Error(w, "You do not have this book borrowed", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM checkouts WHERE user_id = $1 AND book_id = $2", userID, bookID)
	if err != nil {
		http.Error(w, "Error returning the book", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE books SET available_quantity = available_quantity + 1 WHERE id = $1", bookID)
	if err != nil {
		http.Error(w, "Error updating book availability", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Book returned successfully",
		"user_id": userID,
		"book_id": bookID,
	})
}

func listCheckoutsHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT u.id, u.first_name, u.last_name, b.id, b.title 
		FROM checkouts c
		JOIN users u ON c.user_id = u.id
		JOIN books b ON c.book_id = b.id
	`)
	if err != nil {
		http.Error(w, "Error fetching checkouts", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var checkouts []Checkout

	for rows.Next() {
		var checkout Checkout
		err := rows.Scan(&checkout.UserID, &checkout.UserFirstName, &checkout.UserLastName, &checkout.BookID, &checkout.BookTitle)
		if err != nil {
			http.Error(w, "Error scanning rows", http.StatusInternalServerError)
			return
		}
		checkouts = append(checkouts, checkout)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(checkouts)
}

func main() {
	http.HandleFunc("/books", listBooksHandler)
	http.HandleFunc("/checkouts", listCheckoutsHandler)
	http.HandleFunc("/books/borrow", borrowBookHandler)
	http.HandleFunc("/books/return", returnBookHandler)
	http.HandleFunc("/users", listUsersHandler)
	http.HandleFunc("/users/add", addUserHandler)

	fmt.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
