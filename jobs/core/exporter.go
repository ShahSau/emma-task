package core

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gothinkster/golang-gin-realworld-example-app/articles"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
)

// StreamExport writes data from DB to the writer with filters
func StreamExport(resource, format string, filters map[string]string, writer io.Writer) (int, error) {
	db := common.GetDB()
	count := 0

	// CSV Writer setup
	var csvWriter *csv.Writer
	if format == "csv" {
		csvWriter = csv.NewWriter(writer)
		defer csvWriter.Flush()
	}

	// JSON Array setup
	if format == "json" {
		writer.Write([]byte("["))
	}

	encoder := json.NewEncoder(writer)

	// Helper to write a record
	writeRecord := func(data map[string]interface{}, csvHeaders []string) error {
		if format == "csv" {
			if count == 0 {
				if err := csvWriter.Write(csvHeaders); err != nil {
					return err
				}
			}
			var row []string
			for _, h := range csvHeaders {
				val := ""
				if v, ok := data[h]; ok && v != nil {
					val = fmt.Sprintf("%v", v)
				}
				row = append(row, val)
			}
			return csvWriter.Write(row)
		} else if format == "json" {
			if count > 0 {
				writer.Write([]byte(","))
			}
			return encoder.Encode(data)
		} else {
			return encoder.Encode(data)
		}
	}

	switch resource {
	case "users":
		query := db.Model(&users.UserModel{})
		// Apply User Filters
		if username, ok := filters["username"]; ok && username != "" {
			query = query.Where("username = ?", username)
		}

		rows, err := query.Rows()
		if err != nil {
			return 0, err
		}
		defer rows.Close()

		headers := []string{"id", "username", "email", "bio", "image"}
		for rows.Next() {
			var u users.UserModel
			db.ScanRows(rows, &u)
			data := map[string]interface{}{
				"id": u.UUID, "username": u.Username, "email": u.Email, "bio": u.Bio, "image": u.Image,
			}
			if err := writeRecord(data, headers); err != nil {
				return count, err
			}
			count++
		}

	case "articles":
		query := db.Model(&articles.ArticleModel{})
		// Apply Article Filters
		if author, ok := filters["author"]; ok && author != "" {
			// Join to find author ID by username
			var user users.UserModel
			if err := db.Where("username = ?", author).First(&user).Error; err == nil {
				query = query.Where("author_id = ?", user.ID)
			}
		}
		if slug, ok := filters["slug"]; ok && slug != "" {
			query = query.Where("slug = ?", slug)
		}

		rows, err := query.Rows()
		if err != nil {
			return 0, err
		}
		defer rows.Close()

		headers := []string{"id", "slug", "title", "description", "body", "created_at", "updated_at", "author_id"}
		for rows.Next() {
			var a articles.ArticleModel
			db.ScanRows(rows, &a)
			data := map[string]interface{}{
				"id": a.UUID, "slug": a.Slug, "title": a.Title, "description": a.Description,
				"body": a.Body, "created_at": a.CreatedAt.Format(time.RFC3339), "updated_at": a.UpdatedAt.Format(time.RFC3339), "author_id": a.AuthorID,
			}
			if err := writeRecord(data, headers); err != nil {
				return count, err
			}
			count++
		}

	case "comments":
		query := db.Model(&articles.CommentModel{})
		// Apply Comment Filters
		if articleSlug, ok := filters["article"]; ok && articleSlug != "" {
			var art articles.ArticleModel
			if err := db.Where("slug = ?", articleSlug).First(&art).Error; err == nil {
				query = query.Where("article_id = ?", art.ID)
			}
		}

		rows, err := query.Rows()
		if err != nil {
			return 0, err
		}
		defer rows.Close()

		headers := []string{"id", "body", "article_id", "author_id", "created_at"}
		for rows.Next() {
			var c articles.CommentModel
			db.ScanRows(rows, &c)
			data := map[string]interface{}{
				"id": c.ID, "body": c.Body, "article_id": c.ArticleID, "author_id": c.AuthorID, "created_at": c.CreatedAt.Format(time.RFC3339),
			}
			if err := writeRecord(data, headers); err != nil {
				return count, err
			}
			count++
		}

	default:
		return 0, fmt.Errorf("unknown resource: %s", resource)
	}

	if format == "json" {
		writer.Write([]byte("]"))
	}
	return count, nil
}
