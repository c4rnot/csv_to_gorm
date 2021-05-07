package main

import (
	"log"
	"os"

	"github.com/c4rnot/csv_to_gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Apple struct {
	gorm.Model // include ID, CretedAt, UpdatedAt, DeletedAt
	Name       string
	Diameter   float64
	Popularity float64
	Origin     string
	Discovered uint
	ForCooking bool
	ForEating  bool
}

var (
	colMap = map[string]int{
		"Name":       1,
		"Diameter":   2,
		"Popularity": 3,
		"Origin":     4,
		"Discovered": 5,
		"ForCooking": 6,
		"ForEating":  7,
	}
)

func main() {
	// connect to the DB
	dsn := os.Getenv("DATABASE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		CreateBatchSize: 1000,
	})
	if err != nil {
		log.Fatal("failed to connect database ", err)
	}

	// Migrate the schema
	err = db.AutoMigrate(&Apple{})
	if err != nil {
		log.Fatal("could not migrate schema ", err)
	}

	var apple Apple

	// note, you need to typecast the returned interface so that Gorm knows what it is
	apples := csv_to_gorm.CsvToSlice("apples.csv", &apple, colMap).([]Apple)
	db.Create(&apples)

}
