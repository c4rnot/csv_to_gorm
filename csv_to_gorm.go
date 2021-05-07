package csv_to_gorm

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// converts the content of a CSV file to a slice of 'models'
// colMap maps the feldnames of the model to the column numbers (beginning at 1) of the CSV file
// the result is an interface, which will need to be typecast by the caller
func CsvToSlice(file string, model interface{}, colMap map[string]int) interface{} {

	// determine what type of model we are trying to fill records of
	modelTyp := reflect.ValueOf(model).Elem().Type()
	modelNumFlds := modelTyp.NumField()

	// make an empty slice to hold the records to be uploaded to the db.
	// ***  FIRST DETERMINE HOW BIG THE ARRAY HAS TO BE  **
	//numRecords := 27308
	//objArryTyp := reflect.ArrayOf(numRecords, modelTyp)
	//objArry := reflect.New(objArryTyp).Elem()
	objSlice := reflect.Zero(reflect.SliceOf(modelTyp))

	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	//r.Comma = ';'

	// read the first line separately as it contains the headings
	colNames, err := r.Read()
	if err != nil {
		log.Fatal("Cannot read heading row of CSV file")
	}
	fmt.Println("Number of Columns: ", len(colNames))
	fmt.Println("fields per record: ", r.FieldsPerRecord)
	fmt.Println("Column Headings: ", colNames)

	var recordIx int = 0
	// for each line of the CSV file, which is a record
	dbRecordPtr := reflect.New(modelTyp)
	for {
		if recordIx%1000 == 0 {
			fmt.Println("Begin Record procedure. Record No.", recordIx)
		}
		csvRecord, err := r.Read()
		if err == io.EOF {
			fmt.Println("EOF Reached")
			break
		}

		// create the new item to add to the database
		dbRecordPtr = reflect.New(modelTyp)

		// for each field in the model
		for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
			fldName := modelTyp.Field(fldIx).Name
			fldType := modelTyp.Field(fldIx).Type

			csvCol := colMap[fldName] - 1
			if csvCol >= r.FieldsPerRecord {
				log.Fatal("Column supplied in map is out of range")
			}
			// if a csv column maps to the field
			if csvCol >= 0 {
				dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[csvCol], fldType))
			}
		}
		// add the record to the slice of records
		// objArry.Index(recordIx).Set(reflect.ValueOf(dbRecordPtr.Elem().Interface()))
		objSlice = reflect.Append(objSlice, dbRecordPtr.Elem())

		// incriment index and go to next record
		recordIx++
	}
	return objSlice.Interface()
}

// takes the text string of a CSV field and converts it to a reflect.Value of a given type (supplied as a reflect.Type)
// used internally, but exposed as it may have uses elsewhere
func StringToType(input string, outType reflect.Type) reflect.Value {
	switch outType.Kind() {
	case reflect.String:
		rtnString := strings.ToValidUTF8(input, "")
		return reflect.ValueOf(rtnString)
	case reflect.Bool:
		//fmt.Println("Step a: bool")
		if strings.ContainsAny(input[0:2], "YyTt1") || strings.Contains(strings.ToLower(input), "true") || strings.Contains(strings.ToLower(input), "yes") {
			return reflect.ValueOf(true)
		} else {
			return reflect.ValueOf(false)
		}
	case reflect.Int, reflect.Uint, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		result := reflect.New(reflect.Type(outType))

		i, err := strconv.Atoi(input)
		if err != nil {
			log.Fatal("stringToType could not convert "+input+" to integer: ", err)
		} else {
			if outType.Kind() == reflect.Int || outType.Kind() == reflect.Int64 || outType.Kind() == reflect.Int32 || outType.Kind() == reflect.Int16 || outType.Kind() == reflect.Int8 {
				result.Elem().SetInt(int64(i))
				return result.Elem()
			} else {
				result.Elem().SetUint(uint64(i))
				return result.Elem()
			}
		}
	case reflect.Float32, reflect.Float64:
		resultPtr := reflect.New(reflect.Type(outType))
		var bitSize int
		if outType.Kind() == reflect.Float64 {
			bitSize = 64
		} else {
			bitSize = 32
		}

		f, err := strconv.ParseFloat(input, bitSize)
		if err != nil {
			// we could be trying to read a %
			f, err = strconv.ParseFloat(strings.Replace(input, "%", "", 1), bitSize)
			if err != nil {
				// we could be reading a German encoded file with the decimals represented as commas
				f, err = strconv.ParseFloat(strings.Replace(input, ",", ".", 1), bitSize)
				if err != nil {
					// could be German format and %
					f, err = strconv.ParseFloat(strings.Replace(strings.Replace(input, ",", ".", 1), "%", "", 1), bitSize)
					if err != nil {
						// not for lack of trying
						log.Fatal("stringToType could not convert "+input+" to float: ", err)
					}
					// number was %, so divide by 100
					f = f / 100.0
				}
			}
			// number was %, so divide by 100
			f = f / 100.0
		}
		resultPtr.Elem().SetFloat(f)
		return resultPtr.Elem()
	default:
		fmt.Println("Step a: default")
		log.Fatal("stringToType has recieved a ", outType, " and does not kow how to handle it")
	}
	return reflect.ValueOf(errors.New("stringToType could not convert type " + outType.Name()))
}

// configurations that need implimenting
// -------------------------------------
// * which separator character the encoding uses
// * which decimal format is used
// * whether % needs to divide the number by 100 or not

// routines which need implimenting
// -------------------------------------
// * date and time conversions
// * populating structs containing structs
