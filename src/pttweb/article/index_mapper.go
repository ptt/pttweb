package article

const (
	kIndexMapperPrealloc = 16
)

type IndexMapper struct {
	indexArr []int
	elemLen  int
}

func NewIndexMapper(elemLen int) *IndexMapper {
	L := elemLen * kIndexMapperPrealloc
	return &IndexMapper{
		indexArr: make([]int, L, L),
		elemLen:  elemLen,
	}
}

func (im *IndexMapper) Reset() {
}

func (im *IndexMapper) Record(from int, to ...int) {
	for im.elemLen*from+im.elemLen >= len(im.indexArr) {
		L := len(im.indexArr)
		newArr := make([]int, 2*L, 2*L)
		copy(newArr, im.indexArr)
		im.indexArr = newArr
	}
	for i := 0; i < im.elemLen; i++ {
		im.indexArr[from*im.elemLen+i] = to[i]
	}
}

func (im *IndexMapper) Get(from int) []int {
	return im.indexArr[from*im.elemLen : (from+1)*im.elemLen]
}
