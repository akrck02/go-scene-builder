package mesh

// Instances are many copies of one model: the model's pieces (around its origin, in scene units)
// are shared, and every copy has a position, a turn around the vertical, a size and a tint.
type Instances struct {
	Name      string
	Model     []*Piece
	Positions []float32 // x, y, z of every copy
	Turns     []float32 // radians around the vertical
	Scales    []float32
	Tints     []float32 // linear RGBA multiplied with the model's colours
}

// Add places one more copy.
func (instances *Instances) Add(position Vector, turn, scale float64, tint [4]float32) {
	instances.Positions = append(instances.Positions, float32(position[0]), float32(position[1]), float32(position[2]))
	instances.Turns = append(instances.Turns, float32(turn))
	instances.Scales = append(instances.Scales, float32(scale))
	instances.Tints = append(instances.Tints, tint[:]...)
}

// Count is the number of copies.
func (instances *Instances) Count() int { return len(instances.Turns) }

// Append adds the copies of other instances of the same model.
func (instances *Instances) Append(other *Instances) {
	instances.Positions = append(instances.Positions, other.Positions...)
	instances.Turns = append(instances.Turns, other.Turns...)
	instances.Scales = append(instances.Scales, other.Scales...)
	instances.Tints = append(instances.Tints, other.Tints...)
}
