// Package media implements the RentMy media domain: photo uploads, S3 storage, and thumbnail generation.
package media

import "time"

// MediaType categorises what a photo represents.
type MediaType string

const (
	MediaTypeListingPhoto MediaType = "LISTING_PHOTO"
	MediaTypeCheckIn      MediaType = "CHECK_IN"
	MediaTypeCheckOut     MediaType = "CHECK_OUT"
	MediaTypeKYCID        MediaType = "KYC_ID"
	MediaTypeKYCSelfie    MediaType = "KYC_SELFIE"
)

// Media is the domain representation of a stored image.
type Media struct {
	ID               string     `json:"id"`
	ListingID        *string    `json:"listingId,omitempty"`
	TransactionID    *string    `json:"transactionId,omitempty"`
	MediaType        MediaType  `json:"mediaType"`
	OriginalURL      string     `json:"originalUrl"`
	ThumbnailURL     *string    `json:"thumbnailUrl,omitempty"`
	OrientationRoll  *float32   `json:"orientationRoll,omitempty"`
	OrientationPitch *float32   `json:"orientationPitch,omitempty"`
	OrientationYaw   *float32   `json:"orientationYaw,omitempty"`
	GpsLat           *float32   `json:"gpsLat,omitempty"`
	GpsLng           *float32   `json:"gpsLng,omitempty"`
	DeviceID         *string    `json:"deviceId,omitempty"`
	CapturedAt       *time.Time `json:"capturedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// Orientation holds gyroscope/accelerometer Euler angles captured at shutter press.
type Orientation struct {
	Roll  *float32 `json:"roll"`
	Pitch *float32 `json:"pitch"`
	Yaw   *float32 `json:"yaw"`
}

// UploadInput carries all metadata accompanying a raw image upload.
type UploadInput struct {
	// MediaType defaults to LISTING_PHOTO when empty.
	MediaType     MediaType    `json:"mediaType"`
	Orientation   *Orientation `json:"orientation"`
	GpsLat        *float32     `json:"gpsLat"`
	GpsLng        *float32     `json:"gpsLng"`
	DeviceID      *string      `json:"deviceId"`
	CapturedAt    *time.Time   `json:"capturedAt"`
}
