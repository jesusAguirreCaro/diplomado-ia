package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	dataframe "github.com/kfultz07/go-dataframe"
)

// Función para manejar errores y evitar repetición de código
func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Contar líneas en el archivo
func LineCounter(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}

func FragCsvtoCsv(inpFn string) {

	// inpDir := "./dat/in/"
	// outDir := "./dat/out/"

	//Abrir el archivo de entrada para contar las líneas
	filePath := inpFn + ".csv"
	fc, err := os.Open(filePath)
	checkError(err)
	defer fc.Close()

	//var rc io.Reader = fc

	totInpLns, err := LineCounter(fc) //total input lines
	checkError(err)
	fmt.Println("Number of lines in file =", totInpLns)
	fmt.Println("continue")

	// open input file again, this time for reading content
	f, err := os.Open(filePath)
	checkError(err)
	defer f.Close()

	//var r io.Reader = f
	csvReader := csv.NewReader(f)

	// Definir el número de archivos de salida
	nf := 10
	nRowsOutFile := totInpLns / nf
	if totInpLns%nf != 0 {
		nRowsOutFile++
	}

	header, err := csvReader.Read()
	checkError(err)

	//Proceso de fragmentación
	outFmt := ".csv"
	inpLnCt := 0 //Contador de lineas de entrada

	for i := 0; i < nf; i++ {
		outFn := inpFn + "_" + strconv.Itoa(i) + outFmt
		fmt.Println("FileName = ", outFn)

		csvFile, err := os.Create(outFn)
		checkError(err)
		defer csvFile.Close()

		w := csv.NewWriter(csvFile)
		//defer w.Flush()
		w.Write(header)

		for j := 0; j < nRowsOutFile; j++ {
			inpLnCt++
			rc, err := csvReader.Read()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				log.Fatal(err)
			}
			w.Write(rc)
		}

		w.Flush()
		checkError(w.Error())
		//defer csvFile.Close() // file will be closed when the function exits
		fmt.Println("inpLnCnt=", inpLnCt)
	}

}

func ReadMoviesCsvFile(filename string) dataframe.DataFrame {

	path := "./"

	df := dataframe.CreateDataFrame(path, filename)

	return df
}

func ReadRatingsCsvFile(filename string) dataframe.DataFrame {

	path := "./"

	df := dataframe.CreateDataFrame(path, filename)

	return df
}

func Mt_FindRatingsMaster() {
	fmt.Println("In MtFindRatingsMaster")
	start := time.Now()
	nf := 10 // number of files with ratings is also a number of threads for multi-threading

	// kg is a 1D array that contains the Known Genres
	kg := []string{"Action", "Adventure", "Animation", "Children", "Comedy", "Crime", "Documentary",
		"Drama", "Fantasy", "Film-Noir", "Horror", "IMAX", "Musical", "Mystery", "Romance",
		"Sci-Fi", "Thriller", "War", "Western", "(no genres listed)"}

	ng := len(kg) //number of known genres
	/* ra is a 2D array where the ratings values for each genre are maintained.
	The columns signal/maintain the core number where a worker is running
	The rows in that column maintain the raiting values for that core and that genre */
	ra := make([][]float64, ng)
	/* ca is a 2D array where the count of Ratings for each genre are maintained.
	The columns signal the core number where the worker is running
	The rows in that column maintain the counts for that genre */
	ca := make([][]int, ng)
	// populate the ng rows of ra and ca with nf columns
	for i := 0; i < ng; i++ {
		ra[i] = make([]float64, nf)
		ca[i] = make([]int, nf)
	}
	var ci = make(chan int) // create the channel to sync all workers
	movies := ReadMoviesCsvFile("movies.csv")
	// run FindRatings in 10 workers
	for i := 0; i < nf; i++ {
		go Mt_FindRatingsWorker(i+1, ci, kg, &ca, &ra, movies)
	}

	//wait for the workers
	iMsg := 0
	go func() {
		for {
			i := <-ci
			iMsg += i
		}
	}()
	for {
		if iMsg == nf {
			break
		}
	}
	// all workers completed their work. Collect results and produce report
	locCount := make([]int, ng)
	locVals := make([]float64, ng)
	for i := 0; i < ng; i++ {
		for j := 0; j < nf; j++ {
			locCount[i] += ca[i][j]
			locVals[i] += ra[i][j]
		}
	}

	fmt.Printf("%-3s %-20s %10s %15s\n", "ID", "Genre", "Count", "Average Rating")
	fmt.Println(strings.Repeat("-", 50))

	for i := 0; i < ng; i++ {
		if locCount[i] > 0 {
			avgRating := locVals[i] / float64(locCount[i])
			fmt.Printf("%-3d %-20s %10d %15.2f\n", i, kg[i], locCount[i], avgRating)
		} else {
			fmt.Printf("%-3d %-20s %10d %15s\n", i, kg[i], locCount[i], "No ratings")
		}

	}

	duration := time.Since(start)
	fmt.Println("Duration = ", duration)
	println("Mt_FindRatingsMaster is Done")
}

func Mt_FindRatingsWorker(w int, ci chan int, kg []string, ca *[][]int, va *[][]float64, movies dataframe.DataFrame) {
	aFileName := "ratings_" + fmt.Sprintf("%d", w-1) + ".csv"
	println("Worker  ", fmt.Sprintf("%02d", w-1), " is processing file ", aFileName, "\n")

	ratings := ReadRatingsCsvFile(aFileName)
	ng := len(kg)
	start := time.Now()

	//import all records from the movies DF into the ratings DF, keeping genres column from movies
	// df.Merge is the equivalent of an inner-join in the DF lib I am using here
	ratings.Merge(&movies, "movieId", "genres")

	// We only need "genres" and "ratings" to find Count(Ratings | Generes), so keep only those columns
	grcs := [2]string{"genres", "rating"} // grcs => Genres Ratings Columns
	grDF := ratings.KeepColumns(grcs[:])  // grDF => Genres Ratings DF
	for ig := 0; ig < ng; ig++ {
		for _, row := range grDF.FrameRecords {
			if strings.Contains(row.Data[0], kg[ig]) {
				(*ca)[ig][w-1] += 1
				v, _ := strconv.ParseFloat((row.Data[1]), 32) // do not check for error
				(*va)[ig][w-1] += v
			}
		}
	}
	duration := time.Since(start)
	fmt.Println("Duration = ", duration)
	fmt.Println("Worker ", w, " completed")

	// notify master that this worker has completed its job

	ci <- 1
}

func main() {
	fmt.Println("FileFrag Start")
	FragCsvtoCsv("ratings")
	fmt.Println("FileFrag End")
	Mt_FindRatingsMaster()
}