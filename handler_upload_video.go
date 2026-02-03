package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/calculateAspect"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (1 <<30))
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid JWT", err)
		return
	}
	
	log.Println("uploading video for", videoID, "by user", userID)

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unable to get video", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	
	mediaData, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaData != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", err)
		return
	}

	videoTemp, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer videoTemp.Close()

	_, err = io.Copy(videoTemp, file)
	
	_, err = videoTemp.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to rest video", err)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to generate rand key", err)
		return
	}
	hexFileKey := hex.EncodeToString(randomBytes)
	finalFileKey := hexFileKey + ".mp4"

	params := &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key: 		aws.String(finalFileKey),
		Body: 	videoTemp,
		ContentType: aws.String(mediaData),
	}

	_, err = cfg.s3Client.PutObject(context.TODO(), params)

	videoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, finalFileKey)
	videoData.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}

func getvideoAspectRatio(filepath string) (string, error) {
	type videoAspect struct {
		Width		string 	`json:"width"`
		Height 	string	`json:"height"`
	}
	videoCmd := exec.Command("ffprobe", "-v error", "-print_format json", "-show_stream ./samples/boots-video-vertical.mp4")
	var bytesBuff bytes.Buffer
	videoCmd.Stdout = &bytesBuff
	videoCmd.Run()


	var videoAsp *videoAspect
	err := json.Unmarshal(bytesBuff.Bytes(), &videoAsp)
	if err != nil {
		return "", fmt.Errorf("Error unmarshalling, %v", err)
	}

	calculatedAspect := IndentifyAspectRatio(videoAsp.Width, videoAsp.Height)
}