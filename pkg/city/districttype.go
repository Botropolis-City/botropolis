package city

// DistrictType is what kind of buildings a district grows; a building's
// kind follows its storeys within it (item 109).
type DistrictType string

const (
	ResidentialDistrict DistrictType = "residential"
	ShoppingDistrict    DistrictType = "shopping"
	OfficeDistrict      DistrictType = "office"
	IndustrialDistrict  DistrictType = "industrial"
)

// DistrictTypes are the four, in the order the city offers them.
var DistrictTypes = []DistrictType{ResidentialDistrict, ShoppingDistrict, OfficeDistrict, IndustrialDistrict}
