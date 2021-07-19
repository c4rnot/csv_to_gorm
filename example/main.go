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

	applesFileName    = "apples.csv"
	orangesFileName   = "oranges.csv"
	yieldFileName     = "yield.csv"
	lossesFileName    = "pest_losses.csv"
	exportersFileName = "biggest_exporters.csv"
)

type Orange struct {
	gorm.Model         // include ID, CretedAt, UpdatedAt, DeletedAt
	Name       string  `xtg:"col:Name"`
	Diameter   float64 `xtg:"col:diameter"`
	Popularity float64 `xtg:"col:Liked By"`
}

type Yield struct {
	gorm.Model         // include ID, CretedAt, UpdatedAt, DeletedAt
	Name       string  `xtg:"col:Name"`
	Product    string  `xtg:"mapConst:product"`
	Year       int     `xtg:"intcols:colname"`
	Yield      float64 `xtg:"intcols:value"`
}

type PestLoss struct {
	gorm.Model         // include ID, CretedAt, UpdatedAt, DeletedAt
	Name       string  `xtg:"col:Name"`
	Cause      string  `xtg:"melt:colname"`
	Loss       float64 `xtg:"melt:value"`
}

type BiggestExporter struct {
	gorm.Model             // include ID, CretedAt, UpdatedAt, DeletedAt
	Country         string `xtg:"col:country"`
	Type            string `xtg:"melt:colname"`
	ExportCode      int    `xtg:"melt:value"`
	Year            int    `xtg:"intcols:colname"`
	BiggestExporter string `xtg:"intcols:value"`
}

func main() {

	// open the file to read and defer its closure
	applesFile, err := os.Open(applesFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer applesFile.Close()

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
	db.AutoMigrate(&Orange{})
	db.AutoMigrate(&Yield{})
	db.AutoMigrate(&PestLoss{})
	db.AutoMigrate(&BiggestExporter{})

	params := csv_to_gorm.Params{
		ColMap:          colMap,
		FirstRowHasData: false,
	}

	// ** Guess separator example
	// *******************************
	sep, err := csv_to_gorm.GuessSeparator(applesFile)
	if err != nil {
		fmt.Println("Error guessing Separator", err.Error())
	}
	fmt.Println("Separator guessed to be: ", string(sep))

	// ** Get headings example
	// *******************************
	headings, err := csv_to_gorm.GetHeadings(applesFile, sep)
	if err != nil {
		fmt.Println("Errorgetting headings", err.Error())
	}
	fmt.Println("CSV file column headings are: : ", headings)

	// ** Get Database fields example
	// *******************************
	dbFields, err := csv_to_gorm.GetDbFields(&Apple{})
	if err != nil {
		fmt.Println("Errorgetting headings", err.Error())
	}
	fmt.Println("Database Fields are: : ", dbFields)

	// ** Read csv to database example
	// *******************************

	// note, you need to typecast the returned interface so that Gorm knows what it is
	apples, err := csv_to_gorm.CsvToSlice(applesFile, sep, &Apple{}, params)
	apples = apples.([]Apple)
	if err != nil {
		fmt.Println("Error creating Apples", err)
	}
	fmt.Println("apples", apples)
	db.Model(&Apple{}).Create(apples) // PASS

	params = csv_to_gorm.Params{
		FirstRowHasData: false,
	}
	orangesFile, err := os.Open(orangesFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer orangesFile.Close()
	oranges, err := csv_to_gorm.CsvToSlice(orangesFile, sep, &Orange{}, params)
	oranges = oranges.([]Orange)
	if err != nil {
		fmt.Println("Error creating Oranges", err)
	}
	db.Model(&Orange{}).Create(oranges) // PASS

	params = csv_to_gorm.Params{
		FirstRowHasData: false,
		ConstMap:        map[string]string{"product": "apple"},
	}
	yieldFile, err := os.Open(yieldFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer yieldFile.Close()
	yields, err := csv_to_gorm.CsvToSlice(yieldFile, sep, &Yield{}, params)
	yields = yields.([]Yield)
	if err != nil {
		fmt.Println("Error creating yields", err)
	}
	db.Model(&Yield{}).Create(yields) // PASS

	params = csv_to_gorm.Params{
		FirstRowHasData: false,
	}
	lossesFile, err := os.Open(lossesFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer lossesFile.Close()
	pestLosses, err := csv_to_gorm.CsvToSlice(lossesFile, sep, &PestLoss{}, params)
	pestLosses = pestLosses.([]PestLoss)
	if err != nil {
		fmt.Println("Error creating pest_loses", err)
	}
	db.Model(&PestLoss{}).Create(pestLosses) // PASS

	params = csv_to_gorm.Params{
		FirstRowHasData: false,
	}

	exportersFile, err := os.Open(exportersFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer exportersFile.Close()
	biggestExporters, err := csv_to_gorm.CsvToSlice(exportersFile, sep, &BiggestExporter{}, params)
	biggestExporters = biggestExporters.([]BiggestExporter)
	if err != nil {
		fmt.Println("Error creating exporters", err)
	}
	db.Model(&BiggestExporter{}).Create(biggestExporters) // PASS

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
