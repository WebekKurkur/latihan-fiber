package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"latihan-fiber/app/model"
)

var (
	ErrPortfolioProjectLimit     = errors.New("batas proyek portofolio tercapai")
	ErrPortfolioNotFound         = errors.New("portofolio atau proyek tidak ditemukan")
	ErrPortfolioInvalidProjectID = errors.New("id proyek tidak valid")
)

const MaxPortfolioProjects = 20

type PortfolioRepository interface {
	EnsureIndexes(context.Context) error
	FindByStudentID(context.Context, int) (model.StudentPortfolio, error)
	Replace(context.Context, model.StudentPortfolio) (model.StudentPortfolio, error)
	AddProject(context.Context, int, model.PortfolioProject) (model.StudentPortfolio, error)
	DeleteProject(context.Context, int, string) error
	TopTechnologies(context.Context) ([]model.TechnologyStat, error)
	FindAllAfterCursor(context.Context, model.CursorQuery) ([]model.StudentPortfolio, error)
}

type portfolioDocument struct {
	StudentID    int                       `bson:"student_id"`
	Headline     string                    `bson:"headline"`
	Bio          string                    `bson:"bio"`
	Projects     []portfolioProjectDoc     `bson:"projects"`
	Certificates []portfolioCertificateDoc `bson:"certificates"`
	UpdatedAt    time.Time                 `bson:"updated_at"`
}

type portfolioProjectDoc struct {
	ID           bson.ObjectID `bson:"_id"`
	Title        string        `bson:"title"`
	Technologies []string      `bson:"technologies"`
	Link         string        `bson:"link"`
	Year         int           `bson:"year"`
}

type portfolioCertificateDoc struct {
	Name     string    `bson:"name"`
	Issuer   string    `bson:"issuer"`
	IssuedAt time.Time `bson:"issued_at"`
}

type portfolioMongoRepository struct{ coll *mongo.Collection }

func NewPortfolioRepository(db *mongo.Database) PortfolioRepository {
	return &portfolioMongoRepository{coll: db.Collection("student_portfolios")}
}

func (r *portfolioMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "student_id", Value: 1}}, Options: options.Index().SetUnique(true).SetName("student_id_unique")},
		{Keys: bson.D{{Key: "projects.technologies", Value: 1}}, Options: options.Index().SetName("project_technologies")},
	})
	if err != nil {
		return fmt.Errorf("membuat index student_portfolios: %w", err)
	}
	return nil
}

func (r *portfolioMongoRepository) FindByStudentID(ctx context.Context, studentID int) (model.StudentPortfolio, error) {
	var doc portfolioDocument
	if err := r.coll.FindOne(ctx, bson.D{{Key: "student_id", Value: studentID}}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.StudentPortfolio{}, ErrPortfolioNotFound
		}
		return model.StudentPortfolio{}, fmt.Errorf("membaca portofolio: %w", err)
	}
	return doc.toModel(), nil
}

// FindAllAfterCursor mengambil satu halaman daftar seluruh portofolio dengan
// keyset pagination.
//
// Urutannya adalah (updated_at, student_id) DESC. student_id ikut dibandingkan
// karena updated_at TIDAK dijamin unik: bila dua portofolio diperbarui pada
// milidetik yang sama dan hanya updated_at yang dibandingkan, salah satu akan
// terlewat atau terkirim dua kali.
//
// Jumlah yang diminta sengaja limit+1. Baris tambahan itu tidak dikirim ke
// client; keberadaannya hanya dipakai untuk menjawab "masih ada halaman
// berikutnya?" tanpa perlu COUNT(*) atas seluruh collection.
//
// Pagination memakai aggregation (bukan Find) supaya urutan dan filter kunci
// dapat dinyatakan dalam satu pipeline tanpa bergantung pada API opsi Find.
func (r *portfolioMongoRepository) FindAllAfterCursor(
	ctx context.Context, q model.CursorQuery,
) ([]model.StudentPortfolio, error) {
	conditions := bson.A{}

	if q.Search != "" {
		// QuoteMeta dipakai agar karakter khusus regex yang diketik klien
		// tidak mengubah arti pola pencarian.
		pattern := regexp.QuoteMeta(q.Search)
		conditions = append(conditions, bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "headline", Value: bson.D{{Key: "$regex", Value: pattern}, {Key: "$options", Value: "i"}}}},
			bson.D{{Key: "bio", Value: bson.D{{Key: "$regex", Value: pattern}, {Key: "$options", Value: "i"}}}},
		}}})
	}

	if q.After != nil {
		// model.Cursor dipakai ulang: CreatedAt berarti updated_at, ID
		// berarti student_id.
		conditions = append(conditions, bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "updated_at", Value: bson.D{{Key: "$lt", Value: q.After.CreatedAt}}}},
			bson.D{
				{Key: "updated_at", Value: q.After.CreatedAt},
				{Key: "student_id", Value: bson.D{{Key: "$lt", Value: q.After.ID}}},
			},
		}}})
	}

	// Semua kondisi digabung dalam satu $and agar tidak ada dua operator
	// $or pada dokumen yang sama — MongoDB menolak dokumen query yang
	// memuat kunci operator yang sama dua kali.
	match := bson.D{}
	if len(conditions) > 0 {
		match = bson.D{{Key: "$and", Value: conditions}}
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "updated_at", Value: -1}, {Key: "student_id", Value: -1}}}},
		bson.D{{Key: "$limit", Value: q.Limit + 1}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar portofolio: %w", err)
	}
	defer cursor.Close(ctx)

	result := make([]model.StudentPortfolio, 0)
	for cursor.Next(ctx) {
		var doc portfolioDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("membaca daftar portofolio: %w", err)
		}
		result = append(result, doc.toModel())
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("membaca cursor daftar portofolio: %w", err)
	}
	return result, nil
}

func (r *portfolioMongoRepository) Replace(ctx context.Context, portfolio model.StudentPortfolio) (model.StudentPortfolio, error) {
	doc := portfolioDocumentFromModel(portfolio)
	doc.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	_, err := r.coll.ReplaceOne(ctx,
		bson.D{{Key: "student_id", Value: portfolio.StudentID}}, doc,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return model.StudentPortfolio{}, fmt.Errorf("mengganti portofolio: %w", err)
	}
	return doc.toModel(), nil
}

func (r *portfolioMongoRepository) AddProject(ctx context.Context, studentID int, project model.PortfolioProject) (model.StudentPortfolio, error) {
	if project.ID == "" {
		project.ID = bson.NewObjectID().Hex()
	}
	projectID, err := bson.ObjectIDFromHex(project.ID)
	if err != nil {
		return model.StudentPortfolio{}, ErrPortfolioInvalidProjectID
	}
	projectDoc := portfolioProjectDoc{ID: projectID, Title: project.Title, Technologies: project.Technologies, Link: project.Link, Year: project.Year}
	filter := bson.D{{Key: "student_id", Value: studentID}, {Key: "$expr", Value: bson.D{{Key: "$lt", Value: bson.A{bson.D{{Key: "$size", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$projects", bson.A{}}}}}}, MaxPortfolioProjects}}}}}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "projects", Value: projectDoc}}},
		{Key: "$set", Value: bson.D{{Key: "updated_at", Value: time.Now().UTC().Truncate(time.Millisecond)}}},
	}
	var doc portfolioDocument
	if err := r.coll.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			current, lookupErr := r.FindByStudentID(ctx, studentID)
			if lookupErr == nil && len(current.Projects) >= MaxPortfolioProjects {
				return model.StudentPortfolio{}, ErrPortfolioProjectLimit
			}
			return model.StudentPortfolio{}, ErrPortfolioNotFound
		}
		return model.StudentPortfolio{}, fmt.Errorf("menambahkan proyek portofolio: %w", err)
	}
	return doc.toModel(), nil
}

func (r *portfolioMongoRepository) DeleteProject(ctx context.Context, studentID int, projectID string) error {
	id, err := bson.ObjectIDFromHex(projectID)
	if err != nil {
		return ErrPortfolioInvalidProjectID
	}
	result, err := r.coll.UpdateOne(ctx,
		bson.D{{Key: "student_id", Value: studentID}, {Key: "projects._id", Value: id}},
		bson.D{
			{Key: "$pull", Value: bson.D{{Key: "projects", Value: bson.D{{Key: "_id", Value: id}}}}},
			{Key: "$set", Value: bson.D{{Key: "updated_at", Value: time.Now().UTC().Truncate(time.Millisecond)}}},
		},
	)
	if err != nil {
		return fmt.Errorf("menghapus proyek portofolio: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrPortfolioNotFound
	}
	return nil
}

func (r *portfolioMongoRepository) TopTechnologies(ctx context.Context) ([]model.TechnologyStat, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$unwind", Value: "$projects"}},
		bson.D{{Key: "$unwind", Value: "$projects.technologies"}},
		bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$projects.technologies"}, {Key: "usage_count", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "usage_count", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: 10}},
	}
	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mengagregasi teknologi portofolio: %w", err)
	}
	defer cursor.Close(ctx)
	result := make([]model.TechnologyStat, 0)
	for cursor.Next(ctx) {
		var row struct {
			Technology string `bson:"_id"`
			UsageCount int    `bson:"usage_count"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, fmt.Errorf("membaca agregasi teknologi: %w", err)
		}
		result = append(result, model.TechnologyStat{Technology: row.Technology, UsageCount: row.UsageCount})
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("membaca cursor teknologi: %w", err)
	}
	return result, nil
}

func portfolioDocumentFromModel(portfolio model.StudentPortfolio) portfolioDocument {
	doc := portfolioDocument{
		StudentID: portfolio.StudentID, Headline: portfolio.Headline, Bio: portfolio.Bio,
		Projects:     make([]portfolioProjectDoc, 0, len(portfolio.Projects)),
		Certificates: make([]portfolioCertificateDoc, 0, len(portfolio.Certificates)),
		UpdatedAt:    portfolio.UpdatedAt,
	}
	for _, project := range portfolio.Projects {
		id, err := bson.ObjectIDFromHex(project.ID)
		if err != nil || project.ID == "" {
			id = bson.NewObjectID()
		}
		doc.Projects = append(doc.Projects, portfolioProjectDoc{ID: id, Title: project.Title, Technologies: project.Technologies, Link: project.Link, Year: project.Year})
	}
	for _, cert := range portfolio.Certificates {
		doc.Certificates = append(doc.Certificates, portfolioCertificateDoc{Name: cert.Name, Issuer: cert.Issuer, IssuedAt: cert.IssuedAt.UTC().Truncate(time.Millisecond)})
	}
	return doc
}

func (d portfolioDocument) toModel() model.StudentPortfolio {
	portfolio := model.StudentPortfolio{
		StudentID: d.StudentID, Headline: d.Headline, Bio: d.Bio,
		Projects:     make([]model.PortfolioProject, 0, len(d.Projects)),
		Certificates: make([]model.PortfolioCertificate, 0, len(d.Certificates)),
		UpdatedAt:    d.UpdatedAt,
	}
	for _, project := range d.Projects {
		portfolio.Projects = append(portfolio.Projects, model.PortfolioProject{ID: project.ID.Hex(), Title: project.Title, Technologies: project.Technologies, Link: project.Link, Year: project.Year})
	}
	for _, cert := range d.Certificates {
		portfolio.Certificates = append(portfolio.Certificates, model.PortfolioCertificate{Name: cert.Name, Issuer: cert.Issuer, IssuedAt: cert.IssuedAt})
	}
	return portfolio
}
