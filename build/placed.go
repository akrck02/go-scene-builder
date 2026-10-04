package build

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/style"
)

// model places a model of the library on every point of a feature (the "model" role):
//
//	shape: {role: model, model: farol, scale: 1, turn: random}   # turn: degrees, or random
func (builder *featureBuilder) model(feature *mapdata.Feature, shape style.Style) error {
	model := builder.input.Models.Model(shape.Text("model", ""))
	if model == nil {
		return fmt.Errorf("no model %q in the libraries", shape.Text("model", ""))
	}
	random := rand.New(rand.NewPCG(style.Seed(feature.Properties, "model"), 3))
	for _, point := range feature.Points {
		scale, turn, err := sizeAndTurn(shape, feature.Properties, random)
		if err != nil {
			return err
		}
		position := builder.frame.Scene(point[0], point[1], builder.height(point[0], point[1]))
		chosen, factor, tint := model.Copy(random)
		builder.output.copiesOf(chosen, point[0], point[1]).Add(position, turn, scale*factor, tint)
	}
	return nil
}

// sizeAndTurn reads "scale" (a number, an expression or a range [low, high]) and "turn" (degrees or
// "random") for one copy.
func sizeAndTurn(settings style.Style, properties map[string]any, random *rand.Rand) (float64, float64, error) {
	scale := 1.0
	if low, high := rangeOf(settings["scale"], math.NaN(), math.NaN()); !math.IsNaN(low) {
		scale = low + (high-low)*random.Float64()
	} else if value, err := settings.Number("scale", properties, 1); err != nil {
		return 0, 0, err
	} else {
		scale = value
	}
	if settings.Text("turn", "") == "random" {
		return scale, random.Float64() * 2 * math.Pi, nil
	}
	turn, err := settings.Number("turn", properties, 0)
	return scale, turn * math.Pi / 180, err
}

// weightedModels reads a recipe's models: {name: weight, …}, the models found in the library.
func weightedModels(settings style.Style, library *models.Library) ([]*models.Model, []float64, []string) {
	weights, _ := settings["models"].(map[string]any)
	var chosen []*models.Model
	var shares []float64
	var missing []string
	for name, value := range weights {
		weight, _ := toNumber(value)
		if model := library.Model(name); model != nil && weight > 0 {
			chosen = append(chosen, model)
			shares = append(shares, weight)
		} else if model == nil {
			missing = append(missing, name)
		}
	}
	return chosen, shares, missing
}

func pickWeighted(models []*models.Model, weights []float64, random *rand.Rand) *models.Model {
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	target := random.Float64() * total
	for index, weight := range weights {
		if target < weight {
			return models[index]
		}
		target -= weight
	}
	return models[len(models)-1]
}
