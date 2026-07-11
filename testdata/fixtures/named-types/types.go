package namedtypes

type Item struct {
	Name string
}

type Sizer interface {
	Len() int
}

type Items []Item

func (items Items) Len() int {
	return len(items)
}

type Index map[string]*Item

type Transform func(Items) Index

type Code int

type Vector [4]Item

type ItemAlias = Item

type hidden []Item

type secret struct {
	Item Item
}
