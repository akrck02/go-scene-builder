package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

// Rivers always run below the ground around them. Before the terrain is built, every water feature
// with lines (a river axis) and a width digs its channel into a copy of the relief: along the axis
// the water lies "freeboard" metres under the lower bank, never rising downstream, and the bed is
// "depth" metres under the water. The ground meets the water at the river's edge (or three quarters
// of a relief cell from the axis, if the river is narrower: a coarse grid cannot draw a narrower
// one) and rises over it to the banks one cell farther; the water sheet covers the whole trench, so
// where the banks rise above it the ground hides it.
// Bridges ask the channels how high the water is under them.

const (
	defaultDepth     = 2.0 // metres from the water to the bed
	defaultFreeboard = 0.6 // metres from the lower bank down to the water
)

// channelSample is a point of a river axis with its water level.
type channelSample struct {
	point mapdata.Point
	level float64 // metres
}

// channel is one water feature dug into the relief.
type channel struct {
	samples   []channelSample
	halfWidth float64 // km
	wet       float64 // km from the axis to where the ground meets the water
	reach     float64 // km from the axis that the trench and the water sheet cover
	depth     float64 // metres
	freeboard float64 // metres
}

// channels are the rivers of a map, by feature, with an index of their samples.
type channels struct {
	byFeature map[*mapdata.Feature]*channel
	cell      float64
	cells     map[[2]int][]indexedSample
}

type indexedSample struct {
	channel *channel
	sample  int
}

// liveSampleSpacing is the distance (km) between the samples of a river axis without a relief.
const liveSampleSpacing = 0.005

// digChannels finds the rivers of the map and lowers a copy of the relief under them. Without a
// relief (a live scene drawn on a terrain that exists already) nothing can be dug: the water lies
// "above_bed" metres over the ground of its axis, as the terrain drew it, and the channels only
// tell bridges where it is (the returned relief is nil).
func digChannels(input Input) (*channels, *relief.Relief) {
	found := &channels{byFeature: map[*mapdata.Feature]*channel{}, cells: map[[2]int][]indexedSample{}}
	for _, feature := range input.Map.Features {
		if current := newChannel(feature, input); current != nil {
			found.byFeature[feature] = current
			found.cell = math.Max(found.cell, current.reach)
		}
	}
	if len(found.byFeature) == 0 {
		return nil, nil
	}
	if input.Relief == nil {
		found.index(input.Map.Features)
		return found, nil
	}
	found.index(input.Map.Features)
	return found, found.dig(input.Relief)
}

// index puts the samples in cells, in the map's order (a repeatable index).
func (found *channels) index(features []*mapdata.Feature) {
	for _, feature := range features {
		if current := found.byFeature[feature]; current != nil {
			for index, sample := range current.samples {
				key := found.cellOf(sample.point)
				found.cells[key] = append(found.cells[key], indexedSample{current, index})
			}
		}
	}
}

func newChannel(feature *mapdata.Feature, input Input) *channel {
	if len(feature.Lines) == 0 {
		return nil
	}
	resolved, matched := input.Rules.Resolve(feature.Kind, feature.Properties)
	if !matched {
		return nil
	}
	shape := resolved.Section("shape")
	if shape.Text("role", "") != "water" {
		return nil
	}
	width, err := shape.Number("width", feature.Properties, 0)
	if err != nil || width <= 0 {
		return nil
	}
	depth, err := shape.Number("depth", feature.Properties, defaultDepth)
	if err != nil {
		return nil
	}
	freeboard, err := shape.Number("freeboard", feature.Properties, defaultFreeboard)
	if err != nil {
		return nil
	}
	halfWidth := width / 2000
	ground := input.Relief
	if ground == nil {
		return liveChannel(feature, input, shape, halfWidth)
	}
	wet := math.Max(halfWidth, 0.75*ground.Cell)
	current := &channel{halfWidth: halfWidth, wet: wet, reach: wet + ground.Cell, depth: depth, freeboard: freeboard}
	for _, line := range feature.Lines {
		current.samples = append(current.samples, levelsAlong(densified(line, ground.Cell/2), halfWidth+ground.Cell, freeboard, ground)...)
	}
	return current
}

// liveChannel is a river on a terrain that exists already: its water "above_bed" metres over the
// ground of its axis, as wide as the river.
func liveChannel(feature *mapdata.Feature, input Input, shape style.Style, halfWidth float64) *channel {
	aboveBed, err := shape.Number("above_bed", feature.Properties, 1)
	if err != nil {
		return nil
	}
	height := func(east, north float64) float64 {
		if input.Heights != nil {
			return input.Heights.Height(east, north)
		}
		return 0
	}
	current := &channel{halfWidth: halfWidth, wet: halfWidth, reach: halfWidth}
	for _, line := range feature.Lines {
		for _, point := range densified(line, liveSampleSpacing) {
			current.samples = append(current.samples, channelSample{point, height(point[0], point[1]) + aboveBed})
		}
	}
	return current
}

// levelsAlong puts the water under the lower bank of each point of an axis, and never lets it rise
// on its way down: the end whose water is lower is the mouth.
func levelsAlong(axis []mapdata.Point, bankDistance, freeboard float64, ground *relief.Relief) []channelSample {
	samples := make([]channelSample, len(axis))
	for index, point := range axis {
		across := acrossAt(axis, index)
		lowest := ground.Height(point[0], point[1])
		for _, side := range []float64{-1, 1} {
			bank := mapdata.Point{point[0] + side*bankDistance*across[0], point[1] + side*bankDistance*across[1]}
			lowest = math.Min(lowest, ground.Height(bank[0], bank[1]))
		}
		samples[index] = channelSample{point, lowest - freeboard}
	}
	if len(samples) > 1 && samples[0].level < samples[len(samples)-1].level {
		for index := len(samples) - 2; index >= 0; index-- {
			samples[index].level = math.Min(samples[index].level, samples[index+1].level)
		}
	} else {
		for index := 1; index < len(samples); index++ {
			samples[index].level = math.Min(samples[index].level, samples[index-1].level)
		}
	}
	return samples
}

// acrossAt is the unit vector across an axis at one of its points.
func acrossAt(axis []mapdata.Point, index int) [2]float64 {
	before, after := axis[max(index-1, 0)], axis[min(index+1, len(axis)-1)]
	east, north := after[0]-before[0], after[1]-before[1]
	length := math.Hypot(east, north)
	if length == 0 {
		return [2]float64{1, 0}
	}
	return [2]float64{-north / length, east / length}
}

// dig returns a copy of the relief with every channel's trench: the bed across the river, rising to
// the banks at the edge of the reach.
func (found *channels) dig(ground *relief.Relief) *relief.Relief {
	dug := *ground
	dug.Heights = append([]float32(nil), ground.Heights...)
	for _, current := range found.byFeature {
		for _, sample := range current.samples {
			current.digAround(&dug, sample)
		}
	}
	return &dug
}

func (current *channel) digAround(dug *relief.Relief, sample channelSample) {
	firstColumn := int(math.Floor((sample.point[0]-current.reach-dug.West)/dug.Cell - 0.5))
	lastColumn := int(math.Ceil((sample.point[0]+current.reach-dug.West)/dug.Cell - 0.5))
	firstRow := int(math.Floor((dug.North-sample.point[1]-current.reach)/dug.Cell - 0.5))
	lastRow := int(math.Ceil((dug.North-sample.point[1]+current.reach)/dug.Cell - 0.5))
	for row := max(firstRow, 0); row <= min(lastRow, dug.Rows-1); row++ {
		for column := max(firstColumn, 0); column <= min(lastColumn, dug.Columns-1); column++ {
			east, north := dug.CellCentre(row, column)
			distance := math.Hypot(east-sample.point[0], north-sample.point[1])
			if distance > current.reach {
				continue
			}
			index := row*dug.Columns + column
			dug.Heights[index] = float32(math.Min(float64(dug.Heights[index]), current.trench(sample.level, distance)))
		}
	}
}

// trench is the height of the dug ground at a distance (km) from the axis: a bowl from the bed up
// to the water at the wet edge, then a slope up to the banks, over the water, at the reach.
func (current *channel) trench(level, distance float64) float64 {
	if distance <= current.wet {
		share := distance / current.wet
		return level - current.depth*(1-share*share)
	}
	return level + current.freeboard*(distance-current.wet)/(current.reach-current.wet)
}

// of is the channel of a feature, or nil.
func (found *channels) of(feature *mapdata.Feature) *channel {
	if found == nil {
		return nil
	}
	return found.byFeature[feature]
}

// trenchSurfaces are a river's polygons and its axis widened to the reach of its trench.
func (builder *featureBuilder) trenchSurfaces(feature *mapdata.Feature, current *channel) []*geos.Geom {
	var polygons []*geos.Geom
	for _, polygon := range feature.Polygons {
		polygons = append(polygons, builder.context.NewPolygon(ringsOf(polygon)).MakeValid())
	}
	for _, line := range feature.Lines {
		// flat ends: past the ends of its axis a river has no trench (and its mouth meets other water)
		polygons = append(polygons, builder.context.NewLineString(coordinatesOf(line)).BufferWithStyle(current.reach, 4, geos.BufCapStyleFlat, geos.BufJoinStyleRound, 5))
	}
	return builder.insideTheRelief(polygons)
}

func (found *channels) cellOf(point mapdata.Point) [2]int {
	return [2]int{int(math.Floor(point[0] / found.cell)), int(math.Floor(point[1] / found.cell))}
}

// levelOf is the water level of a channel at a point: that of its nearest sample (the water sheet
// lies within reach of the axis, so the cells around the point hold it).
func (found *channels) levelOf(current *channel, point mapdata.Point) float64 {
	centre := found.cellOf(point)
	best, bestDistance := current.samples[0].level, math.Inf(1)
	for column := centre[0] - 1; column <= centre[0]+1; column++ {
		for row := centre[1] - 1; row <= centre[1]+1; row++ {
			for _, entry := range found.cells[[2]int{column, row}] {
				if entry.channel != current {
					continue
				}
				sample := current.samples[entry.sample]
				if distance := mapdata.Distance(sample.point, point); distance < bestDistance {
					best, bestDistance = sample.level, distance
				}
			}
		}
	}
	return best
}

// waterUnder is the highest water within reach of a point, and whether there is any.
func (found *channels) waterUnder(point mapdata.Point) (float64, bool) {
	if found == nil {
		return 0, false
	}
	centre := found.cellOf(point)
	highest, any := math.Inf(-1), false
	for column := centre[0] - 1; column <= centre[0]+1; column++ {
		for row := centre[1] - 1; row <= centre[1]+1; row++ {
			for _, entry := range found.cells[[2]int{column, row}] {
				sample := entry.channel.samples[entry.sample]
				if mapdata.Distance(sample.point, point) <= entry.channel.reach {
					highest, any = math.Max(highest, sample.level), true
				}
			}
		}
	}
	return highest, any
}
