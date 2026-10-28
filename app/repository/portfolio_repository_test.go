package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"latihan-fiber/app/model"
)

func TestPortfolioRepositoryMongoIntegration(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetServerSelectionTimeout(2 * time.Second))
	if err != nil {
		t.Skipf("MongoDB tidak tersedia: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB tidak tersedia: %v", err)
	}

	db := client.Database("portfolio_test_" + time.Now().Format("20060102150405000000000"))
	defer func() { _ = db.Drop(context.Background()) }()
	repo := NewPortfolioRepository(db)
	if err := repo.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	portfolio, err := repo.Replace(ctx, model.StudentPortfolio{StudentID: 781223, Headline: "Backend engineer", Bio: "Membangun aplikasi dan layanan", Certificates: []model.PortfolioCertificate{{Name: "Go Basics", Issuer: "Example", IssuedAt: time.Now()}}})
	if err != nil {
		t.Fatal(err)
	}
	if portfolio.StudentID != 781223 {
		t.Fatalf("student_id = %d, want 781223", portfolio.StudentID)
	}
	if _, err := repo.Replace(ctx, model.StudentPortfolio{StudentID: 781223, Headline: "Updated", Bio: "Portofolio diperbarui"}); err != nil {
		t.Fatal(err)
	}

	seed := model.StudentPortfolio{StudentID: 781224, Headline: "Developer", Bio: "Portofolio untuk uji limit"}
	if _, err := repo.Replace(ctx, seed); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 18; i++ {
		project := model.PortfolioProject{ID: fmtObjectID(), Title: "Existing", Technologies: []string{"Go"}, Year: 2025}
		if _, err := repo.AddProject(ctx, seed.StudentID, project); err != nil {
			t.Fatal(err)
		}
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			requestCtx, requestCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer requestCancel()
			project := model.PortfolioProject{ID: fmtObjectID(), Title: "Concurrent", Technologies: []string{"Go"}, Year: 2026}
			_, err := repo.AddProject(requestCtx, seed.StudentID, project)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrPortfolioProjectLimit) {
				t.Errorf("AddProject error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if got := successes.Load(); got != 2 {
		t.Fatalf("concurrent project additions succeeded %d times, want 2", got)
	}
	portfolio, err = repo.FindByStudentID(ctx, seed.StudentID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(portfolio.Projects); got != MaxPortfolioProjects {
		t.Fatalf("project count = %d, want %d", got, MaxPortfolioProjects)
	}
	stats, err := repo.TopTechnologies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) == 0 || stats[0].Technology != "Go" {
		t.Fatalf("unexpected technology stats: %#v", stats)
	}
	if err := repo.DeleteProject(ctx, seed.StudentID, portfolio.Projects[0].ID); err != nil {
		t.Fatal(err)
	}
}

func fmtObjectID() string {
	return bson.NewObjectID().Hex()
}
