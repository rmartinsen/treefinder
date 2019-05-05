package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type resp struct {
	Kind  string
	Items []item
}

type item struct {
	Title string
	Link  string
}

type s3Handler struct {
	Session *session.Session
	Bucket  string
}

type SSM struct {
	*client.Client
}

func main() {
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		startNum := i * 10
		fmt.Print(startNum)
		searchToS3("quercus lobata", startNum, &wg)
		// searchToS3("Cornus kousa", startNum, &wg)
		// searchToS3("Magnolia Grandiflora", startNum, &wg)
		// searchToS3("Prunus Okame", startNum, &wg)
	}

	wg.Wait()
}

func searchToS3(text string, start int, wg *sync.WaitGroup) {
	wg.Add(1)

	var wgInner sync.WaitGroup
	resp := new(resp)
	url := queryString(text, start)
	responseFromURL(url, resp)
	for i, item := range resp.Items {
		wgInner.Add(1)
		fileID := start + i
		s3Path := outputPathFromText(text, fileID)
		imageURL := item.Link
		go urlToS3Object(s3Path, imageURL, &wgInner)
	}
	wgInner.Wait()
	wg.Done()
}

func uploadImageToS3(body io.Reader, filename string) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	if err != nil {
		fmt.Println(err)
	}

	handler := s3Handler{
		Session: sess,
		Bucket:  "treefinder",
	}

	err = handler.UploadFile(filename, body)
	if err != nil {
		fmt.Println(err)
	}

}

func (h s3Handler) UploadFile(key string, body io.Reader) error {
	bodyBytes, err := ioutil.ReadAll(body)

	_, err = s3.New(h.Session).PutObject(&s3.PutObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(key),
		ACL:    aws.String("private"),
		Body:   bytes.NewReader(bodyBytes),
	})

	return err
}

func queryString(queryText string, start int) string {
	formattedInput := strings.ReplaceAll(queryText, " ", "+")
	queryBase := "https://www.googleapis.com/customsearch/v1?q=%s&num=10&start=%d&imgSize=medium&searchType=image&cx=011992137466501229235:-0nok_uw7ly&key=%s"
	apiKey := getAPIKey()
	queryWithInput := fmt.Sprintf(queryBase, formattedInput, start, apiKey)
	return queryWithInput
}

func getAPIKey() string {
	sess, err := session.NewSession(&aws.Config{Region: aws.String("us-east-1")})
	if err != nil {
		fmt.Println(err)
	}

	client := ssm.New(sess)

	paramName := "google_api_key"
	withDecryption := true

	input := &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: &withDecryption}
	paramOutput, err := client.GetParameter(input)
	if err != nil {
		fmt.Println(err)
	}

	return *paramOutput.Parameter.Value

}

func outputPathFromText(text string, fileID int) string {
	outputBase := "validation"
	formattedText := strings.ReplaceAll(text, " ", "_")
	return fmt.Sprintf("%s/%s/%d.jpg", outputBase, formattedText, fileID)
}

func responseFromURL(url string, target interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	return json.NewDecoder(response.Body).Decode(target)
}

func urlToS3Object(outputPath string, url string, wg *sync.WaitGroup) {
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Could not get url %s", url)
		wg.Done()
		return
	}
	defer response.Body.Close()

	uploadImageToS3(response.Body, outputPath)

	wg.Done()

}
