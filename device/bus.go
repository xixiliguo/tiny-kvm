package device

type BusDevice interface {
	Read(base uint64, offset uint64, data []byte)
	Write(base uint64, offset uint64, data []byte) error
}
