package main

import (
	"fmt"
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

	//fileName = "apples.csv"
	fileName = "apples_german_enc.csv"
)

func main() {

	// open the file to read and defer its closure
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// connect to the DB
	dsn := os.Getenv("DATABASE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		CreateBatchSize: 1000,
	})
	if err != nil {
		log.Fatal("failed to connect database ", err)
	}

	// Ensure the expected schema is applied to the database
	err = db.AutoMigrate(&Apple{})
	if err != nil {
		log.Fatal("could not migrate schema ", err)
	}

	// ** Guess separator example
	// *******************************
	sep, err := csv_to_gorm.GuessSeparator(file)
	if err != nil {
		fmt.Println("Error guessing Separator", err.Error())
	}
	fmt.Println("Separator guessed to be: ", string(sep))

	// ** Get headings example
	// *******************************
	headings, err := csv_to_gorm.GetHeadings(file, sep)
	if err != nil {
		fmt.Println("Errorgetting headings", err.Error())
	}
	fmt.Println("CSV file column headings are: : ", headings)

	// ** Read csv to database example
	// *******************************
	// created simply so that we know what kind of structure to populate.
	var apple Apple

	// note, you need to typecast the returned interface so that Gorm knows what it is
	apples := csv_to_gorm.CsvToSlice(file, sep, &apple, colMap).([]Apple)
	db.Create(&apples)

	// ** Excel col helpers example **
	// *******************************
	colID := "ABC"
	colNo, err := csv_to_gorm.ExcelColIdToColNo(colID)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("Excel column "+colID, "equates to column number ", colNo)
	colID, _ = csv_to_gorm.ExcelColNoToColId(colNo)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("Excel column number ", colNo, "equates to column ", colID)

}
