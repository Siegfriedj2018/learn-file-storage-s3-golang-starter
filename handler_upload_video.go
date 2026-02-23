package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (1 << 30))
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

	aspectRatio, err := getvideoAspectRatio(videoTemp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to get aspect ratio", err)
		return
	}

	var keyPrefix string
	switch aspectRatio {
	case "16:9":
		keyPrefix = "landscape"
	case "9:16":
		keyPrefix = "portrait"
	default:
		keyPrefix = "other"
	}

	key := getAssetPath(mediaData)
	key = path.Join(keyPrefix, key)

	processedVideo, err := processVideoForFastStart(videoTemp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "encountered an error processing video", err)
		return
	}
	processFile, err := os.Open(processedVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "encountered an error opening processed video", err)
		return
	}

	params := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processFile,
		ContentType: aws.String(mediaData),
	}

	_, err = cfg.s3Client.PutObject(context.TODO(), params)

	videoUrl := fmt.Sprintf("https://%v/%v", cfg.s3CfDistribution, key)
	videoData.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	// videoData, err = cfg.dbVideoToSignedVideo(videoData)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "error encoding video", err)
	// 	return
	// }
	respondWithJSON(w, http.StatusOK, videoData)
}

func processVideoForFastStart(filePath string) (string, error) {
	processing := filePath + ".processing"

	processFastStart := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c", "copy",
		"-movflags",
		"faststart",
		"-f", "mp4",
		processing,
	)
	var stderr bytes.Buffer
	processFastStart.Stderr = &stderr

	err := processFastStart.Run()
	if err != nil {
		log.Printf("Command Log: %v", stderr.String())
		log.Printf("Encountered an error processing video: %v", err)
		return "", err
	}

	return processing, nil
}

// func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
// 	if video.VideoURL == nil {
// 		return video, nil
// 	}
// 	parts := strings.Split(*video.VideoURL, ",")
// 	if len(parts) < 2 {
// 		return video, nil
// 	}
// 	bucket := parts[0]
// 	key := parts[1]
// 	presigned, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
// 	if err != nil {
// 		return video, err
// 	}
// 	video.VideoURL = &presigned
// 	return video, nil
// }

// func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	presignClient := s3.NewPresignClient(s3Client)
// 	presignedUrl, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(bucket),
// 		Key:    aws.String(key),
// 	}, s3.WithPresignExpires(expireTime))
// 	if err != nil {
// 		return "", fmt.Errorf("failed to generate presigned URL: %v", err)
// 	}
// 	return presignedUrl.URL, nil
// }