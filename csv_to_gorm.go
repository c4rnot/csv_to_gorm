package csv_to_gorm

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
)

/*
*Instructions:
* =============
*
*  use struct tags to inform xtg how to handle the translation of column to  variable.
* xtg tags have the following form:
* `xtg:<instruction>:<parameter>,<instruction>:<parameter;parameter;parameter>`
*
* mapConst : parameter is the key to the Params constMap.  Value becomes the constant associated with the key
* col: The column name associated with this field
* intcols:colname  xtg will parse all columns whose column names  can parse as an integer.  A separate database record is created for each one
* intcols:value  This field is the value associated with the column headed by an integer.
* melt:colname  takes all colums not declared with col: and creates a separate record for each
* melt:value  takes value associated with colums not declared with col:
* ignore:  takes a ; separated list of strings.  These columns are ignored for melt
 */

type Tag struct {
	HasTag         bool
	HasColanme     bool
	Colname        string
	IsIntColsHead  bool
	IsIntColsValue bool
	IsMapConst     bool
	ConstMapKey    string
	IsMeltHead     bool
	IsMeltValue    bool
	Ignore         []string
}

type Params struct {
	ColMap          map[string]int    // maps fieldnames to column numbers(starting at 1).  Overrides tagnames if mapping present
	ConstMap        map[string]string // maps from tagname mapConst:Mapfrom to a string constant to be parsed into the field
	FirstRowHasData bool
	ErrorOnNaN      bool
	//ErrorOnInf bool
}

func ParseTag(field reflect.StructField) (Tag, error) {
	var tag Tag

	value, ok := field.Tag.Lookup("xtg")
	if !ok || value == "" {
		tag.HasTag = false
		return tag, nil
	} else {
		tag.HasTag = true
	}
	subTags := strings.Split(value, ",")

	for _, subTag := range subTags {
		subTagElements := strings.Split(subTag, ":")
		switch subTagElements[0] {
		case "col":
			tag.HasColanme = true
			if len(subTagElements) < 2 {
				return tag, errors.New("column name missing for field: " + field.Name + ". should be in the form col:<colname>")
			}
			tag.Colname = subTagElements[1]
		case "mapConst":
			tag.IsMapConst = true
			if len(subTagElements) < 2 {
				return tag, errors.New("constant map key is missing for field : " + field.Name + ". should be in the form mapConst:<mapkey>")
			}
			tag.ConstMapKey = subTagElements[1]
		case "intcols":
			if len(subTagElements) < 2 {
				return tag, errors.New("whether field is heading or value field : " + field.Name + ". should be in the form intcols:colname or intcols:value")
			}
			if strings.ToLower(subTagElements[1]) == "colname" {
				tag.IsIntColsHead = true
				tag.IsIntColsValue = false
			} else {
				tag.IsIntColsHead = false
				tag.IsIntColsValue = true
			}
		case "melt":
			if len(subTagElements) < 2 {
				return tag, errors.New("whether field is heading or value field : " + field.Name + ". should be in the form melt:colname or melt:value")
			}
			if strings.ToLower(subTagElements[1]) == "colname" {
				tag.IsMeltHead = true
				tag.IsMeltValue = false
			} else {
				tag.IsMeltHead = false
				tag.IsMeltValue = true
			}
		case "ignore":
			if len(subTagElements) == 1 {
				continue
			}
			ignoreStrings := strings.Split(subTagElements[1], ";")
			tag.Ignore = ignoreStrings
		}
	}
	return tag, nil
}

// converts the content of a CSV file to a slice of 'models'
// colMap maps the feldnames of the model to the column numbers (beginning at 1) of the CSV file
// the result is an interface, which will need to be typecast by the caller
func CsvToSlice(file *os.File, colSep rune, model interface{}, params Params) (dataSlice interface{}, err error) {
	// map of column headings to 1 based column numbers (for consistency with csv_to_gorm)
	var lclColMap map[string]int
	var definedCols []string
	var hasIntCols bool
	var intColHdgs []string
	var hasMelt bool
	var meltColHdgs []string
	var ignore []string
	var errs error
	var maxCol int

	// make sure we start at teh start of the file
	file.Seek(0, 0)

	// determine what type of model we are trying to fill records of
	modelTyp := reflect.ValueOf(model).Elem().Type()
	modelNumFlds := modelTyp.NumField()

	// make an empty slice to hold the records to be uploaded to the db.
	// ***  TODO  Speed up by FIRST DETERMINE HOW BIG THE ARRAY HAS TO BE  **
	objSlice := reflect.Zero(reflect.SliceOf(modelTyp))

	r := csv.NewReader(file)
	r.Comma = colSep

	// read the first line and ignore it , as it contains the headers
	_, err = r.Read()
	if err != nil {
		log.Fatal("CsvToSlice: Cannot read heading row of CSV file")
	}

	var recordIx int = 0
	// for each line of the CSV file, which is a record
	dbRecordPtr := reflect.New(modelTyp)
	for {
		if recordIx%1000 == 0 {
			fmt.Println("Processing record No.", recordIx)
		}
		csvRecord, err := r.Read()
		if err == io.EOF {
			fmt.Println("Reached end of input file")
			break
		}

		// Get headings from first row if necessary
		if recordIx == 0 {
			if !params.FirstRowHasData {
				lclColMap = mapHeadingToCol(csvRecord)
				intColHdgs = getIntCols(csvRecord)
				maxCol = len(csvRecord) - 1

				// check if there is an intcol tag, as a db entry has to be made for each int col
				for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
					tag, err := ParseTag(modelTyp.Field(fldIx))
					if err != nil {
						errs = fmt.Errorf("could not parse tag for :  "+modelTyp.Name()+". ", errs)
					}
					if tag.IsIntColsHead || tag.IsIntColsValue {
						hasIntCols = true
					}
					if tag.IsMeltHead || tag.IsMeltValue {
						hasMelt = true
					}
					if len(tag.Ignore) > 0 {
						ignore = append(ignore, tag.Ignore...)
					}
					if tag.HasColanme {
						definedCols = append(definedCols, tag.Colname)
					}
				}
				if !hasMelt {
					meltColHdgs = []string{}
				} else {
					meltColHdgs = getMeltCols(csvRecord, params.ColMap, definedCols, ignore, hasIntCols, intColHdgs)
				}
			}
			// any other special first row code for case where where data is in first row here
		}

		// create the new item to add to the database
		dbRecordPtr = reflect.New(modelTyp)

		if hasIntCols && !hasMelt {
			for _, intColHdg := range intColHdgs {
				// create the new item to add to the database
				dbRecordPtr = reflect.New(modelTyp)

				// for each field in the model
				for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
					fld := modelTyp.Field(fldIx)
					fldName := fld.Name
					fldType := fld.Type
					tag, _ := ParseTag(fld)

					paramsCol := params.ColMap[fldName]
					//lclCol := lclColMap
					if paramsCol > maxCol {
						log.Fatal("Column supplied in map is out of range")
					}
					// if a parameter column maps to the field
					if paramsCol > 0 {
						cell := csvRecord[paramsCol-1]
						dbRecordPtr.Elem().Field(fldIx).Set(StringToType(cell, fldType, params))
					} else {
						switch {
						case tag.IsMapConst:
							constString := params.ConstMap[tag.ConstMapKey]
							// trying to convert empty strings to numbers in csv_to_gorm will bomb!
							if constString == "" && fldType.Name() != "string" {
								errs = fmt.Errorf("tag constant: "+tag.ConstMapKey+" missing for :  "+modelTyp.Name()+". ", errs)
							} else {
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(constString, fldType, params))
							}
						case tag.IsIntColsHead:
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(intColHdg, fldType, params))
						case tag.IsIntColsValue:
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[intColHdg]-1], fldType, params))
						case tag.HasColanme:
							if lclColMap[tag.Colname] == 0 {
								fmt.Println("Could not find column header " + tag.Colname + " in: " + modelTyp.Name() + ". ")
								errs = fmt.Errorf("Could not find column header "+tag.Colname+" in: "+modelTyp.Name()+". ", errs)
							}
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[tag.Colname]-1], fldType, params))
						}
					}
				}
				// add the record to the slice of records
				objSlice = reflect.Append(objSlice, dbRecordPtr.Elem())
			}

		} else if hasMelt && !hasIntCols {
			for _, meltColHdg := range meltColHdgs {
				// create the new item to add to the database
				dbRecordPtr = reflect.New(modelTyp)

				// for each field in the model
				for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
					fld := modelTyp.Field(fldIx)
					fldName := fld.Name
					fldType := fld.Type
					tag, _ := ParseTag(fld)

					paramsCol := params.ColMap[fldName]
					if paramsCol > maxCol {
						log.Fatal("Column supplied in map is out of range")
					}
					// if a parameter column maps to the field
					if paramsCol > 0 {
						cell := csvRecord[paramsCol-1]
						dbRecordPtr.Elem().Field(fldIx).Set(StringToType(cell, fldType, params))
					} else {
						switch {
						case tag.IsMapConst:
							constString := params.ConstMap[tag.ConstMapKey]
							// trying to convert empty strings to numbers in csv_to_gorm will bomb!
							if constString == "" && fldType.Name() != "string" {
								errs = fmt.Errorf("tag constant: "+tag.ConstMapKey+" missing for :  "+modelTyp.Name()+". ", errs)
							} else {
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(constString, fldType, params))
							}
						case tag.IsMeltHead:
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(meltColHdg, fldType, params))
						case tag.IsMeltValue:
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[meltColHdg]-1], fldType, params))
						case tag.HasColanme:
							if lclColMap[tag.Colname] == 0 {
								fmt.Println("Could not find column header " + tag.Colname + " in :  " + modelTyp.Name() + ". ")
								errs = fmt.Errorf("Could not find column header "+tag.Colname+" in :  "+modelTyp.Name()+". ", errs)
							}
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[tag.Colname]-1], fldType, params))
						}
					}
				}
				// add the record to the slice of records
				// objArry.Index(recordIx).Set(reflect.ValueOf(dbRecordPtr.Elem().Interface()))
				objSlice = reflect.Append(objSlice, dbRecordPtr.Elem())
			}

		} else if hasMelt && hasIntCols {
			for _, intColHdg := range intColHdgs {
				for _, meltColHdg := range meltColHdgs {
					// create the new item to add to the database
					dbRecordPtr = reflect.New(modelTyp)

					// for each field in the model
					for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
						fld := modelTyp.Field(fldIx)
						fldName := fld.Name
						fldType := fld.Type
						tag, _ := ParseTag(fld)

						paramsCol := params.ColMap[fldName]
						if paramsCol > maxCol {
							log.Fatal("Column supplied in map is out of range")
						}
						// if a parameter column maps to the field
						if paramsCol > 0 {
							cell := csvRecord[paramsCol-1]
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(cell, fldType, params))
						} else {
							switch {
							case tag.IsMapConst:
								constString := params.ConstMap[tag.ConstMapKey]
								// trying to convert empty strings to numbers in csv_to_gorm will bomb!
								if constString == "" && fldType.Name() != "string" {
									errs = fmt.Errorf("tag constant: "+tag.ConstMapKey+" missing for :  "+modelTyp.Name()+". ", errs)
								} else {
									dbRecordPtr.Elem().Field(fldIx).Set(StringToType(constString, fldType, params))
								}
							case tag.IsMeltHead:
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(meltColHdg, fldType, params))
							case tag.IsMeltValue:
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[meltColHdg]-1], fldType, params))
							case tag.IsIntColsHead:
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(intColHdg, fldType, params))
							case tag.IsIntColsValue:
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[intColHdg]-1], fldType, params))
							case tag.HasColanme:
								if lclColMap[tag.Colname] == 0 {
									fmt.Println("Could not find column header " + tag.Colname + " in  :  " + modelTyp.Name() + ". ")
									errs = fmt.Errorf("Could not find column header "+tag.Colname+" in  :  "+modelTyp.Name()+". ", errs)
								}
								dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[tag.Colname]-1], fldType, params))
							}
						}
					}
					// add the record to the slice of records
					// objArry.Index(recordIx).Set(reflect.ValueOf(dbRecordPtr.Elem().Interface()))
					objSlice = reflect.Append(objSlice, dbRecordPtr.Elem())
				}
			}

		} else {
			// for each field in the model
			for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
				fld := modelTyp.Field(fldIx)
				fldName := fld.Name
				fldType := fld.Type
				tag, _ := ParseTag(fld)

				paramsCol := params.ColMap[fldName]
				//lclCol := lclColMap
				if paramsCol > maxCol {
					log.Fatal("Column supplied in map is out of range")
				}
				// if a parameter column maps to the field
				if paramsCol > 0 {
					cell := csvRecord[paramsCol-1]
					dbRecordPtr.Elem().Field(fldIx).Set(StringToType(cell, fldType, params))
				} else {
					switch {
					case tag.IsMapConst:
						constString := params.ConstMap[tag.ConstMapKey]
						// trying to convert empty strings to numbers in csv_to_gorm will bomb!
						if constString == "" && fldType.Name() != "string" {
							errs = fmt.Errorf("tag constant: "+tag.ConstMapKey+" missing for :  "+modelTyp.Name()+". ", errs)
						} else {
							dbRecordPtr.Elem().Field(fldIx).Set(StringToType(constString, fldType, params))
						}
					case tag.HasColanme:
						if lclColMap[tag.Colname] == 0 {
							fmt.Println("Could not find column header " + tag.Colname + " in :  " + modelTyp.Name() + ". ")
							errs = fmt.Errorf("Could not find column header "+tag.Colname+" in :  "+modelTyp.Name()+". ", errs)
						}
						dbRecordPtr.Elem().Field(fldIx).Set(StringToType(csvRecord[lclColMap[tag.Colname]-1], fldType, params))
					}
				}
			}
			// add the record to the slice of records
			// objArry.Index(recordIx).Set(reflect.ValueOf(dbRecordPtr.Elem().Interface()))
			objSlice = reflect.Append(objSlice, dbRecordPtr.Elem())
		}

	}
	if err != nil {
		return objSlice.Interface(), err
	}

	return objSlice.Interface(), nil
}

// inputReader would normally be the file or stream to read
func GuessSeparator(file *os.File) (rune, error) {
	// make sure we start at teh start of the file
	file.Seek(0, 0)

	var r *csv.Reader

	// separator candidates in the order in which they will be checked
	sepCandidates := []rune{',', '\t', ';', ' '}

	for _, sep := range sepCandidates {
		//fmt.Println("Trying: '", string(sep), "'")
		isCandidate := true
		// make sure we start at the start of the file each time
		file.Seek(0, 0)
		reader := bufio.NewReader(file)
		line, err := reader.ReadString('\n')
		if err != nil {
			return ',', errors.New("Could not read line of file")
		}
		if !strings.Contains(line, string(sep)) {
			// candidate separator probably not right.  Next one
			isCandidate = false
			continue
		}
		file.Seek(0, 0)
		r = csv.NewReader(file)
		r.Comma = sep

		// read up to the first 100 lines.
		// Flags Error if the number of fields is different form first line or if EOF
		for i := 1; i <= 100; i++ {

			_, err := r.Read()
			if err != nil {
				if err == io.EOF {
					// EOF reached without other error. Separator is probably right
					return sep, nil
				} else {
					// candidate separator probably not right.  Next one
					isCandidate = false
					break
				}
			}
		}
		if isCandidate {
			// EOF reached without other error. Separator is probably right
			return sep, nil
		}
	}
	return ',', errors.New("None of the separators tried were valid")
}

func GetHeadings(file *os.File, colSep rune) ([]string, error) {
	// make sure we start at teh start of the file
	file.Seek(0, 0)

	r := csv.NewReader(file)
	r.Comma = colSep

	// read only the first line
	colNames, err := r.Read()
	if err != nil {
		log.Fatal("GetHeadings: Cannot read heading row of CSV file: " + err.Error())
	}

	return colNames, err
}

func mapHeadingToCol(colNames []string) (colMap map[string]int) {
	colMap = make(map[string]int)
	for colNo, colName := range colNames {
		if colName != "" {
			colMap[colName] = colNo + 1
		}
	}
	return colMap
}

func getIntCols(colNames []string) (intCols []string) {
	for _, colName := range colNames {
		if colName != "" {
			int, err := strconv.Atoi(colName)
			if err != nil {
				intCols = append(intCols, strconv.Itoa(int))
			}
		}
	}
	return intCols
}

func getMeltCols(colNames []string, colMap map[string]int, definedCols []string, ignoreHdgs []string, hasIntCols bool, intColHdgs []string) []string {
	var mappedCols []string
	var meltCols []string

	for _, colName := range colNames {
		for mappedCol := range colMap {
			mappedCols = append(mappedCols, mappedCol)
		}
		// check if column heading is empty
		heading := colName
		if heading == "" {
			continue
		}
		_, isMapped := find(mappedCols, heading)
		if isMapped {
			continue
		}
		_, isDefined := find(definedCols, heading)
		if isDefined {
			continue
		}
		_, isIgnored := find(ignoreHdgs, heading)
		if isIgnored {
			continue
		}
		_, isIntCol := find(intColHdgs, heading)
		if isIntCol && hasIntCols {
			continue
		}

		// if we've not exited the iteration by now, we are a melt column
		meltCols = append(meltCols, heading)
	}
	return meltCols
}

func GetDbFields(model interface{}) ([]string, error) {

	modelTyp := reflect.ValueOf(model).Elem().Type()
	modelNumFlds := modelTyp.NumField()

	var result = make([]string, 0, modelNumFlds)

	// for each field in the model
	for fldIx := 0; fldIx < modelNumFlds; fldIx++ {
		fieldName := modelTyp.Field(fldIx).Name
		if fieldName != "Model" { // don't list the gorm.Model field
			result = append(result, fieldName)
		}
		//fldType := modelTyp.Field(fldIx).Type
	}
	return result, nil
}

// takes the text string of a CSV field and converts it to a reflect.Value of a given type (supplied as a reflect.Type)
// used internally, but exposed as it may have uses elsewhere
func StringToType(input string, outType reflect.Type, params Params) reflect.Value {
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
						if params.ErrorOnNaN {
							log.Fatal("CellToType could not convert "+input+" to float: ", err)
						}
						f = math.NaN()
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

func ExcelColIdToColNo(colID string) (int, error) {
	//firstColNo := 1
	lastColNo := 16384

	var colMap = map[byte]int{
		'A': 1,
		'B': 2,
		'C': 3,
		'D': 4,
		'E': 5,
		'F': 6,
		'G': 7,
		'H': 8,
		'I': 9,
		'J': 10,
		'K': 11,
		'L': 12,
		'M': 13,
		'N': 14,
		'O': 15,
		'P': 16,
		'Q': 17,
		'R': 18,
		'S': 19,
		'T': 20,
		'U': 21,
		'V': 22,
		'W': 23,
		'X': 24,
		'Y': 25,
		'Z': 26,
	}

	colID = strings.ToUpper(strings.Trim(colID, " ,;.:|1234567890"))
	if len(colID) > 3 || len(colID) < 1 {
		return 0, errors.New("Too many or to few characters to be am Excel column ID.  Should be in the form A  or AB or ABC")
	}
	colIDArr := []byte(colID)
	result := 0

	// start with the right most letter and work left
	for iX := len(colIDArr) - 1; iX >= 0; iX-- {

		switch (len(colIDArr) - 1) - iX {
		case 0:
			result = result + colMap[colIDArr[iX]]
		case 1:
			result = result + colMap[colIDArr[iX]]*26
		case 2:
			result = result + colMap[colIDArr[iX]]*26*26
		default:
			return 0, errors.New("index of column ID character out of range")
		}
	}
	if result > lastColNo {
		return 0, errors.New("index of column ID out of range.  Highest column possible is XFD, which equates to column no. " + strconv.Itoa(lastColNo))
	}
	return result, nil
}

func ExcelColNoToColId(colNo int) (string, error) {

	if colNo < 1 {
		return "", errors.New("Colum No. out of range: Too low")
	}
	if colNo > 16384 {
		return "", errors.New("Colum No. out of range: Too high")
	}

	var colMap = map[int]string{
		0:  "",
		1:  "A",
		2:  "B",
		3:  "C",
		4:  "D",
		5:  "E",
		6:  "F",
		7:  "G",
		8:  "H",
		9:  "I",
		10: "J",
		11: "K",
		12: "L",
		13: "M",
		14: "N",
		15: "O",
		16: "P",
		17: "Q",
		18: "R",
		19: "S",
		20: "T",
		21: "U",
		22: "V",
		23: "W",
		24: "X",
		25: "Y",
		26: "Z",
	}

	firstLetter := ""
	secondLetter := ""
	thirdLetter := ""

	if (colNo % 26) == 0 {
		thirdLetter = colMap[26]
	} else {
		thirdLetter = colMap[(colNo % (26))]
	}

	if colNo <= 26 {
		secondLetter = colMap[0]
	} else {
		if int(math.Floor(float64(colNo-1)/26.0))%26 == 0 {
			secondLetter = colMap[26]
		} else {
			secondLetter = colMap[int(int((math.Floor(float64(colNo-1)/26)))%(26))]
			//secondLetter = colMap[int(math.Floor(float64(colNo-1)/(26.0)))]
		}
	}
	if colNo <= 26*27 {
		firstLetter = colMap[0]
	} else {
		firstLetter = colMap[int(math.Floor(float64(colNo-26-1)/(26.0*26.0)))]
	}

	return (firstLetter + secondLetter + thirdLetter), nil
}

// Find takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
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
