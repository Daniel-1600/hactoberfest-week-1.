package retrieval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"ecosound/backend/internal/ai/llm"
	"ecosound/backend/internal/db"
)

var ErrNotFound = errors.New("no curated species record is available")
var ErrModelUnavailable = errors.New("local Qwen model is unavailable")

type Species struct {
	ScientificName          string `json:"scientific_name"`
	CommonName              string `json:"common_name"`
	Habitat                 string `json:"habitat"`
	Diet                    string `json:"diet"`
	EcologicalRole          string `json:"ecological_role"`
	ConservationInformation string `json:"conservation_information"`
	EthicalGuidance         string `json:"ethical_guidance"`
	SourceName              string `json:"source_name"`
	SourceURL               string `json:"source_url"`
	SourceLicense           string `json:"source_license"`
}
type Explanation struct {
	Species
	Explanation       string   `json:"explanation"`
	OutdoorActivities []string `json:"outdoor_activities"`
	Disclaimer        string   `json:"disclaimer"`
}
type Service struct {
	DB      *db.Store
	Model   llm.Client
	Enabled bool
}

func (s Service) Lookup(ctx context.Context, scientificName string) (Species, error) {
	var item Species
	err := s.DB.QueryRowContext(ctx, `SELECT scientific_name,common_name,habitat,diet,ecological_role,conservation_information,ethical_guidance,source_name,source_url,source_license FROM species_knowledge WHERE scientific_name=?`, strings.TrimSpace(scientificName)).Scan(&item.ScientificName, &item.CommonName, &item.Habitat, &item.Diet, &item.EcologicalRole, &item.ConservationInformation, &item.EthicalGuidance, &item.SourceName, &item.SourceURL, &item.SourceLicense)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (s Service) Explain(ctx context.Context, discoveryID int64) (Explanation, error) {
	var scientific, common string
	var score float64
	err := s.DB.QueryRowContext(ctx, `SELECT p.scientific_name,p.common_name,p.confidence FROM discoveries d JOIN predictions p ON p.id=d.prediction_id WHERE d.id=?`, discoveryID).Scan(&scientific, &common, &score)
	if errors.Is(err, sql.ErrNoRows) {
		return Explanation{}, sql.ErrNoRows
	}
	if err != nil {
		return Explanation{}, err
	}
	item, err := s.Lookup(ctx, scientific)
	if err != nil {
		return Explanation{}, err
	}
	if !s.Enabled {
		return Explanation{}, ErrModelUnavailable
	}
	contextData, err := json.Marshal(map[string]any{
		"curated_species_record": item,
		"birdnet_prediction":     map[string]any{"scientific_name": scientific, "common_name": common, "score": score},
	})
	if err != nil {
		return Explanation{}, err
	}
	system := "Explain the curated bird facts and provide exactly three brief, low-impact outdoor nature observation activities. Use species facts only from the provided curated record. The input includes a BirdNET prediction score; it is not proof or a probability of identity. Always describe the bird as a BirdNET suggestion or candidate, never as a confirmed observation. Activities must be general suggestions that do not assume this bird is present. Return one JSON object only, with string field explanation and string array field outdoor_activities. No markdown or extra keys."
	answer, err := s.Model.Complete(ctx, system, string(contextData), true)
	if err != nil {
		return Explanation{}, fmt.Errorf("%w: %v", ErrModelUnavailable, err)
	}
	answer = strings.TrimSpace(answer)
	var generated struct {
		Explanation       string   `json:"explanation"`
		OutdoorActivities []string `json:"outdoor_activities"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(answer))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&generated) != nil || decoder.Decode(new(any)) != io.EOF {
		return Explanation{}, errors.New("model returned an invalid explanation object")
	}
	generated.Explanation = strings.TrimSpace(generated.Explanation)
	if !utf8.ValidString(generated.Explanation) || len(generated.Explanation) > 900 || len(generated.Explanation) < 10 || len(generated.OutdoorActivities) != 3 {
		return Explanation{}, errors.New("model explanation or activities are outside the allowed limits")
	}
	for _, phrase := range []string{"confirmed", "definitely identified", "certainly identified", "proves that"} {
		if strings.Contains(strings.ToLower(generated.Explanation), phrase) {
			return Explanation{}, errors.New("model explanation asserts an unverified identification")
		}
	}
	for i, activity := range generated.OutdoorActivities {
		activity = strings.TrimSpace(activity)
		if !utf8.ValidString(activity) || len(activity) < 10 || len(activity) > 180 {
			return Explanation{}, errors.New("model activity is outside the allowed length")
		}
		generated.OutdoorActivities[i] = activity
	}
	return Explanation{Species: item, Explanation: generated.Explanation, OutdoorActivities: generated.OutdoorActivities, Disclaimer: "BirdNET suggested this species; the recording has not been independently verified. AI text summarizes the cited facts and suggests general outdoor activities."}, nil
}
