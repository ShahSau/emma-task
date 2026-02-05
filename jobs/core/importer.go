package core

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gothinkster/golang-gin-realworld-example-app/articles"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/jobs"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaxErrorLogCount = 1000
	LogInterval      = 5000
	MaxRetries       = 3
)

type RawArticleJSON struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	TagList     []string `json:"tagList"`
	Tags        []string `json:"tags"`
	AuthorName  string   `json:"author"`
	AuthorID    string   `json:"author_id"`
}

type RawCommentJSON struct {
	ID        string    `json:"id"`
	ArticleID string    `json:"article_id"`
	UserID    string    `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type RawUserJSON struct {
	ID, UUID, Email, Username, Name string
}

type JSONArrayReader struct {
	decoder *json.Decoder
	started bool
}

func newJSONArrayReader(reader io.Reader) *JSONArrayReader {
	return &JSONArrayReader{
		decoder: json.NewDecoder(reader),
		started: false,
	}
}

func (r *JSONArrayReader) Read(v interface{}) error {
	if !r.started {
		// Read the opening '['
		tok, err := r.decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok || delim != '[' {
			return fmt.Errorf("expected JSON array starting with '['")
		}
		r.started = true
	}

	if !r.decoder.More() {
		// Read the closing ']'
		_, _ = r.decoder.Token()
		return io.EOF
	}

	return r.decoder.Decode(v)
}

func ProcessImport(job *jobs.Job) error {
	log.Printf(">>> WORKER STARTED processing Job ID: %s", job.ID)

	reader, err := getStreamFromSource(job.SourceKey)
	if err != nil {
		return fmt.Errorf("failed to open stream: %v", err)
	}
	defer reader.Close()

	errFile, err := os.CreateTemp("", fmt.Sprintf("job_errors_%s_*.ndjson", job.ID))
	if err != nil {
		return fmt.Errorf("failed to create error file: %v", err)
	}
	defer os.Remove(errFile.Name())
	defer errFile.Close()

	errorEncoder := json.NewEncoder(errFile)
	var processErr error

	// Basic check for NDJSON extension to decide default strategy,
	// though format detection will handle JSON arrays automatically.
	isNDJSON := strings.HasSuffix(strings.ToLower(job.SourceKey), ".ndjson") ||
		strings.HasSuffix(strings.ToLower(job.SourceKey), ".json")

	switch job.Resource {
	case "users":
		if isNDJSON {
			processErr = importUsersNDJSON(reader, job, errorEncoder)
		} else {
			processErr = importUsersCSV(reader, job, errorEncoder)
		}
	case "articles":
		processErr = importArticlesJSON(reader, job, errorEncoder)
	case "comments":
		processErr = importCommentsJSON(reader, job, errorEncoder)
	default:
		return fmt.Errorf("unknown resource: %s", job.Resource)
	}

	if job.FailedRows > 0 {
		uploadErrorReport(job, errFile.Name())
	}
	log.Printf(">>> Job Finished. Processed: %d, Failed: %d", job.ProcessedRows, job.FailedRows)
	return processErr
}

func getStreamFromSource(source string) (io.ReadCloser, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("remote URL status: %d", resp.StatusCode)
		}
		return resp.Body, nil
	}
	output, err := common.GetS3().GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(source),
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func uploadErrorReport(job *jobs.Job, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	key := fmt.Sprintf("errors/%s.ndjson", job.ID)
	common.GetS3().PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(key),
		Body:   file,
	})
	job.ResultKey = key
}

// detectFormat peeks at the first byte to decide between NDJSON '{' and Array '['
func detectFormat(reader io.Reader) (string, io.Reader, error) {
	bufReader := bufio.NewReader(reader)
	for {
		b, err := bufReader.ReadByte()
		if err != nil {
			return "", nil, err
		}
		// Skip whitespace
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if err := bufReader.UnreadByte(); err != nil {
			return "", nil, err
		}
		switch b {
		case '[':
			return "json_array", bufReader, nil
		case '{':
			return "ndjson", bufReader, nil
		default:
			return "", nil, fmt.Errorf("unknown format, first byte: %c", b)
		}
	}
}

func newLargeScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)
	return scanner
}

func saveBatchWithRetry(db *gorm.DB, batch interface{}) error {
	var err error
	for i := 0; i < MaxRetries; i++ {
		if err = db.CreateInBatches(batch, 1000).Error; err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return err
}

func buildUserModel(raw RawUserJSON) users.UserModel {
	user := users.UserModel{
		Email:        strings.TrimSpace(raw.Email),
		Username:     strings.TrimSpace(raw.Username),
		PasswordHash: "$2a$14$P...",
		UUID:         strings.TrimSpace(raw.ID),
	}
	if user.UUID == "" {
		user.UUID = strings.TrimSpace(raw.UUID)
	}
	if user.Username == "" {
		user.Username = strings.TrimSpace(raw.Name)
	}
	return user
}

func flushUserBatch(db *gorm.DB, users []users.UserModel) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"uuid"}),
	}).CreateInBatches(users, 1000).Error
}

func recordError(job *jobs.Job, writer *json.Encoder, errType, id, msg string) {
	job.FailedRows++
	if job.FailedRows <= MaxErrorLogCount {
		writer.Encode(map[string]string{
			"type":      errType,
			"id":        id,
			"message":   msg,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

func importUsersCSV(reader io.Reader, job *jobs.Job, errWriter *json.Encoder) error {
	db := common.GetDB()
	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("failed header read: %v", err)
	}

	colMap := make(map[string]int)
	for i, h := range header {
		norm := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(h, "\ufeff", ""), "\"", "")))
		colMap[norm] = i
	}

	batchSize := 1000
	var batch []users.UserModel
	batchEmails := make(map[string]bool)

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		user := users.UserModel{PasswordHash: "$2a$14$P..."}
		if idx, ok := colMap["email"]; ok {
			user.Email = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["name"]; ok {
			user.Username = strings.TrimSpace(record[idx])
		} else if idx, ok := colMap["username"]; ok {
			user.Username = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["id"]; ok {
			user.UUID = strings.TrimSpace(record[idx])
		} else if idx, ok := colMap["uuid"]; ok {
			user.UUID = strings.TrimSpace(record[idx])
		}

		if user.Email == "" || user.UUID == "" {
			continue
		}
		if batchEmails[user.Email] {
			continue
		}
		batchEmails[user.Email] = true

		batch = append(batch, user)
		if len(batch) >= batchSize {
			flushUserBatch(db, batch)
			job.ProcessedRows += len(batch)
			batch = nil
			batchEmails = make(map[string]bool)
			if job.ProcessedRows%LogInterval == 0 {
				log.Printf("📊 Progress: %d users", job.ProcessedRows)
			}
		}
	}

	if len(batch) > 0 {
		flushUserBatch(db, batch)
		job.ProcessedRows += len(batch)
	}
	return nil
}

func importUsersNDJSON(reader io.Reader, job *jobs.Job, errWriter *json.Encoder) error {
	db := common.GetDB()

	format, reader, err := detectFormat(reader)
	if err != nil {
		return fmt.Errorf("format detection failed: %v", err)
	}
	log.Printf("✓ Detected format: %s", format)

	batchSize := 1000
	var batch []users.UserModel
	batchEmails := make(map[string]bool)

	// Helper closure to process a single user record
	processUser := func(raw RawUserJSON) {
		user := buildUserModel(raw)
		if user.Email != "" && user.UUID != "" && !batchEmails[user.Email] {
			batchEmails[user.Email] = true
			batch = append(batch, user)
		}

		if len(batch) >= batchSize {
			flushUserBatch(db, batch)
			job.ProcessedRows += len(batch)
			batch = nil
			batchEmails = make(map[string]bool)
			if job.ProcessedRows%LogInterval == 0 {
				log.Printf("📊 Progress: %d users", job.ProcessedRows)
			}
		}
	}

	if format == "ndjson" {
		scanner := newLargeScanner(reader)
		for scanner.Scan() {
			var raw RawUserJSON
			if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
				continue
			}
			processUser(raw)
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	} else {
		arrayReader := newJSONArrayReader(reader)
		for {
			var raw RawUserJSON
			err := arrayReader.Read(&raw)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("⚠️ JSON parse error: %v", err)
				continue
			}
			processUser(raw)
		}
	}

	if len(batch) > 0 {
		flushUserBatch(db, batch)
		job.ProcessedRows += len(batch)
	}

	return nil
}

func importArticlesJSON(reader io.Reader, job *jobs.Job, errWriter *json.Encoder) error {
	db := common.GetDB()

	format, reader, err := detectFormat(reader)
	if err != nil {
		return fmt.Errorf("format detection failed: %v", err)
	}
	log.Printf("✓ Detected format: %s", format)

	authorCache := make(map[string]uint)

	if format == "ndjson" {
		scanner := newLargeScanner(reader)
		return processArticlesNDJSON(scanner, db, job, errWriter, authorCache)
	}

	arrayReader := newJSONArrayReader(reader)
	return processArticlesJSONArray(arrayReader, db, job, errWriter, authorCache)
}

func processArticlesNDJSON(scanner *bufio.Scanner, db *gorm.DB, job *jobs.Job, errWriter *json.Encoder, authorCache map[string]uint) error {
	for scanner.Scan() {
		var raw RawArticleJSON
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		processOneArticle(db, &raw, job, errWriter, authorCache)
	}
	return scanner.Err()
}

func processArticlesJSONArray(arrayReader *JSONArrayReader, db *gorm.DB, job *jobs.Job, errWriter *json.Encoder, authorCache map[string]uint) error {
	for {
		var raw RawArticleJSON
		err := arrayReader.Read(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("⚠️ JSON parse error: %v", err)
			continue
		}
		processOneArticle(db, &raw, job, errWriter, authorCache)
	}
	return nil
}

func processOneArticle(db *gorm.DB, raw *RawArticleJSON, job *jobs.Job, errWriter *json.Encoder, authorCache map[string]uint) bool {
	if raw.Title == "" {
		return false
	}

	tagList := raw.TagList
	if len(tagList) == 0 {
		tagList = raw.Tags
	}

	var articleUserID uint
	if raw.AuthorID != "" {
		if id, ok := authorCache[raw.AuthorID]; ok {
			articleUserID = id
		} else {
			var u users.UserModel
			if err := db.Select("id").Where("uuid = ?", raw.AuthorID).First(&u).Error; err == nil {
				var au articles.ArticleUserModel
				if err := db.Where("user_model_id = ?", u.ID).FirstOrCreate(&au, articles.ArticleUserModel{
					UserModelID: u.ID,
				}).Error; err == nil {
					articleUserID = au.ID
					authorCache[raw.AuthorID] = au.ID
				}
			}
		}
	}

	if articleUserID == 0 {
		recordError(job, errWriter, "DEPENDENCY_ERROR", raw.Slug, "Author not found: "+raw.AuthorID)
		return false
	}

	if raw.Slug == "" {
		raw.Slug = strings.ReplaceAll(strings.ToLower(raw.Title), " ", "-")
	}

	article := articles.ArticleModel{
		Slug:        raw.Slug,
		Title:       raw.Title,
		Description: raw.Description,
		Body:        raw.Body,
		AuthorID:    articleUserID,
		UUID:        raw.ID,
	}

	tx := db.Begin()
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"uuid"}),
	}).Create(&article).Error; err != nil {
		tx.Rollback()
		recordError(job, errWriter, "INSERT_ERROR", raw.Slug, err.Error())
		return false
	}

	if len(tagList) > 0 {
		var tags []articles.TagModel
		for _, tagName := range tagList {
			var tag articles.TagModel
			if err := tx.FirstOrCreate(&tag, articles.TagModel{Tag: tagName}).Error; err != nil {
				tx.Rollback()
				recordError(job, errWriter, "TAG_CREATE_ERROR", raw.Slug, err.Error())
				return false
			}
			tags = append(tags, tag)
		}
		if err := tx.Model(&article).Association("Tags").Replace(tags); err != nil {
			tx.Rollback()
			recordError(job, errWriter, "TAG_LINK_ERROR", raw.Slug, err.Error())
			return false
		}
	}

	if err := tx.Commit().Error; err != nil {
		recordError(job, errWriter, "COMMIT_ERROR", raw.Slug, err.Error())
		return false
	}

	job.ProcessedRows++
	if job.ProcessedRows%LogInterval == 0 {
		log.Printf("📊 Progress: %d articles", job.ProcessedRows)
	}

	return true
}

func importCommentsJSON(reader io.Reader, job *jobs.Job, errWriter *json.Encoder) error {
	db := common.GetDB()

	format, reader, err := detectFormat(reader)
	if err != nil {
		return fmt.Errorf("format detection failed: %v", err)
	}
	log.Printf("✓ Detected format: %s", format)

	articleCache := make(map[string]uint)
	authorCache := make(map[string]uint)
	const batchSize = 1000
	batch := make([]articles.CommentModel, 0, batchSize)

	// Helper closure to process a single comment record
	processComment := func(raw RawCommentJSON) {
		if raw.Body == "" || raw.ArticleID == "" || raw.UserID == "" {
			return
		}

		var articleID uint
		if id, ok := articleCache[raw.ArticleID]; ok {
			articleID = id
		} else {
			var a articles.ArticleModel
			if err := db.Select("id").Where("uuid = ?", raw.ArticleID).First(&a).Error; err == nil {
				articleID = a.ID
				articleCache[raw.ArticleID] = a.ID
			}
		}

		var authorID uint
		if id, ok := authorCache[raw.UserID]; ok {
			authorID = id
		} else {
			var u users.UserModel
			if err := db.Select("id").Where("uuid = ?", raw.UserID).First(&u).Error; err == nil {
				var au articles.ArticleUserModel
				if err := db.Where("user_model_id = ?", u.ID).FirstOrCreate(&au, articles.ArticleUserModel{
					UserModelID: u.ID,
				}).Error; err == nil {
					authorID = au.ID
					authorCache[raw.UserID] = au.ID
				}
			}
		}

		if articleID == 0 || authorID == 0 {
			recordError(job, errWriter, "DEPENDENCY_ERROR", raw.ID, "Missing Article or Author")
			return
		}

		comment := articles.CommentModel{
			Body:      raw.Body,
			ArticleID: articleID,
			AuthorID:  authorID,
		}
		if !raw.CreatedAt.IsZero() {
			comment.CreatedAt = raw.CreatedAt
		}
		batch = append(batch, comment)

		if len(batch) >= batchSize {
			if err := saveBatchWithRetry(db, &batch); err != nil {
				recordError(job, errWriter, "BATCH_ERROR", raw.ID, fmt.Sprintf("Batch failed: %v", err))
			}
			job.ProcessedRows += len(batch)
			batch = batch[:0]
			if job.ProcessedRows%LogInterval == 0 {
				log.Printf("📊 Progress: %d comments", job.ProcessedRows)
			}
		}
	}

	if format == "ndjson" {
		scanner := newLargeScanner(reader)
		for scanner.Scan() {
			var raw RawCommentJSON
			if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
				continue
			}
			processComment(raw)
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	} else {
		arrayReader := newJSONArrayReader(reader)
		for {
			var raw RawCommentJSON
			err := arrayReader.Read(&raw)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("⚠️ JSON parse error: %v", err)
				continue
			}
			processComment(raw)
		}
	}

	if len(batch) > 0 {
		if err := saveBatchWithRetry(db, &batch); err != nil {
			recordError(job, errWriter, "BATCH_ERROR", "FINAL", err.Error())
		}
		job.ProcessedRows += len(batch)
	}

	return nil
}
