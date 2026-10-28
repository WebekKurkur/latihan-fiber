package repository

import (
	"context"
	"errors"
	"fmt"
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
