package listing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeRepo struct {
	listings   map[string]*Listing
	attachedTo map[string][]string // listingID → mediaIDs
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		listings:   make(map[string]*Listing),
		attachedTo: make(map[string][]string),
	}
}

func (f *fakeRepo) Insert(_ context.Context, l *Listing) (*Listing, error) {
	l.CreatedAt = time.Now()
	f.listings[l.ID] = l
	return l, nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*Listing, error) {
	l, ok := f.listings[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}

func (f *fakeRepo) FindByHostID(_ context.Context, hostID string, page, limit int) ([]*Listing, int, error) {
	var results []*Listing
	for _, l := range f.listings {
		if l.HostID == hostID {
			results = append(results, l)
		}
	}
	total := len(results)
	// Apply pagination.
	start := (page - 1) * limit
	if start >= total {
		return nil, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return results[start:end], total, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateListingInput) (*Listing, error) {
	l, ok := f.listings[id]
	if !ok {
		return nil, ErrNotFound
	}
	if in.Title != nil {
		l.Title = *in.Title
	}
	if in.Description != nil {
		l.Description = *in.Description
	}
	if in.PricePerHour != nil {
		l.PricePerHour = in.PricePerHour
	}
	if in.PricePerDay != nil {
		l.PricePerDay = in.PricePerDay
	}
	if in.MaxDuration != nil {
		l.MaxDuration = in.MaxDuration
	}
	if in.Availability != nil {
		l.Availability = in.Availability
	}
	return l, nil
}

func (f *fakeRepo) AttachMedia(_ context.Context, listingID string, mediaIDs []string) error {
	if _, ok := f.listings[listingID]; !ok {
		return ErrNotFound
	}
	f.attachedTo[listingID] = append(f.attachedTo[listingID], mediaIDs...)
	return nil
}

func (f *fakeRepo) UpdateAppraisalFields(_ context.Context, id string, in AppraisalFieldsUpdate) error {
	l, ok := f.listings[id]
	if !ok {
		return ErrNotFound
	}
	l.AppraisalStatus = in.AppraisalStatus
	return nil
}

// --- tests ---

func TestCreate_DefaultsStatusToPending(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{
		Title: "Kayak",
	})
	require.NoError(t, err)

	assert.Equal(t, ListingStatusPending, l.Status)
	assert.Equal(t, "host-1", l.HostID)
	assert.NotEmpty(t, l.ID)
}

func TestCreate_SetsHostID(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	l, err := svc.Create(context.Background(), "user-42", CreateListingInput{
		Title: "Tent",
	})
	require.NoError(t, err)
	assert.Equal(t, "user-42", l.HostID)
}

func TestCreate_RejectsMissingTitle(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	_, err := svc.Create(context.Background(), "host-1", CreateListingInput{})
	require.Error(t, err)
	assert.True(t, isValidationError(err))
}

func TestCreate_Enforces7DayCeiling(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	over := Duration(8 * 24 * time.Hour)
	_, err := svc.Create(context.Background(), "host-1", CreateListingInput{
		Title:       "Kayak",
		MaxDuration: &over,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDurationExceedsLimit)
}

func TestCreate_Allows7DayExact(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	exactly7 := Duration(MaxAllowedDuration)
	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{
		Title:       "Kayak",
		MaxDuration: &exactly7,
	})
	require.NoError(t, err)
	require.NotNil(t, l.MaxDuration)
	assert.Equal(t, exactly7, *l.MaxDuration)
}

func TestCreate_StoresLocation(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{
		Title:    "Kayak",
		Location: &Location{Lat: 33.77, Lng: -118.19},
	})
	require.NoError(t, err)
	require.NotNil(t, l.Location)
	assert.InDelta(t, 33.77, l.Location.Lat, 0.001)
	assert.InDelta(t, -118.19, l.Location.Lng, 0.001)
}

func TestGet_NotFound(t *testing.T) {
	svc := NewServiceWithInterface(newFakeRepo())

	_, err := svc.Get(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestListByHost_PaginationDefaults(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	for i := 0; i < 5; i++ {
		_, err := svc.Create(context.Background(), "host-1", CreateListingInput{
			Title: "Item",
		})
		require.NoError(t, err)
	}

	result, err := svc.ListByHost(context.Background(), "host-1", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Len(t, result.Listings, 5)
}

func TestUpdate_NotOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{Title: "Kayak"})
	require.NoError(t, err)

	newTitle := "Updated"
	_, err = svc.Update(context.Background(), l.ID, "not-host-1", UpdateListingInput{
		Title: &newTitle,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestUpdate_Enforces7DayCeiling(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{Title: "Kayak"})
	require.NoError(t, err)

	over := Duration(8 * 24 * time.Hour)
	_, err = svc.Update(context.Background(), l.ID, "host-1", UpdateListingInput{
		MaxDuration: &over,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDurationExceedsLimit)
}

func TestUpdate_Success(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{Title: "Kayak"})
	require.NoError(t, err)

	newTitle := "Ocean Kayak"
	updated, err := svc.Update(context.Background(), l.ID, "host-1", UpdateListingInput{
		Title: &newTitle,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ocean Kayak", updated.Title)
}

func TestAttachMedia_NotOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{Title: "Kayak"})
	require.NoError(t, err)

	_, err = svc.AttachMedia(context.Background(), l.ID, "other-user", AttachMediaInput{
		MediaIDs: []string{"media-1"},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotOwner)
}

func TestAttachMedia_Success(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithInterface(repo)

	l, err := svc.Create(context.Background(), "host-1", CreateListingInput{Title: "Kayak"})
	require.NoError(t, err)

	_, err = svc.AttachMedia(context.Background(), l.ID, "host-1", AttachMediaInput{
		MediaIDs: []string{"media-1", "media-2"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"media-1", "media-2"}, repo.attachedTo[l.ID])
}

func TestDuration_JSONRoundTrip(t *testing.T) {
	d := Duration(168 * time.Hour)
	data, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"168h0m0s"`, string(data))

	var d2 Duration
	require.NoError(t, json.Unmarshal(data, &d2))
	assert.Equal(t, d, d2)
}

func TestDuration_UnmarshalString(t *testing.T) {
	var d Duration
	require.NoError(t, json.Unmarshal([]byte(`"168h"`), &d))
	assert.Equal(t, Duration(168*time.Hour), d)
}
