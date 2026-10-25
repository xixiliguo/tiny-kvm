package arch

import "github.com/xixiliguo/tiny-kvm/hyper"

// GdtEntry acts as a constructor for a conventional segment GDT (or LDT) entry.
// Derived directly from the Linux kernel's segment.h layouts.
func GdtEntry(flags uint16, base uint32, limit uint32) uint64 {

	return (((uint64(base)) & 0xff000000) << (56 - 24)) |
		(((uint64(flags)) & 0x0000f0ff) << 40) |
		(((uint64(limit)) & 0x000f0000) << (48 - 16)) |
		(((uint64(base)) & 0x00ffffff) << 16) |
		((uint64(limit)) & 0x0000ffff)
}

func getBase(entry uint64) uint64 {
	return (((entry) & 0xFF00000000000000) >> 32) |
		(((entry) & 0x000000FF00000000) >> 16) |
		(((entry) & 0x00000000FFFF0000) >> 16)
}

// GetLimit extracts the segment limit from the GDT segment descriptor.
//
// In a segment descriptor, the limit field is 20 bits, so it can directly describe
// a range from 0 to 0xFFFFF (1MByte). When G flag is set (4-KByte page granularity) it
// scales the value in the limit field by a factor of 2^12 (4Kbytes), making the effective
// limit range from 0xFFF (4 KBytes) to 0xFFFF_FFFF (4 GBytes).
//
// However, the limit field in the VMCS definition is a 32 bit field, and the limit value is not
// automatically scaled using the G flag. This means that for a desired range of 4GB for a
// given segment, its limit must be specified as 0xFFFF_FFFF. Therefore the method of obtaining
// the limit from the GDT entry is not sufficient, since it only provides 20 bits when 32 bits
// are necessary. Fortunately, we can check if the G flag is set when extracting the limit since
// the full GDT entry is passed as an argument, and perform the scaling of the limit value to
// return the full 32 bit value.
//
// The scaling mentioned above is required when using PVH boot, since the guest boots in protected
// (32-bit) mode and must be able to access the entire 32-bit address space. It does not cause issues
// for the case of direct boot to 64-bit (long) mode, since in 64-bit mode the processor does not
// perform runtime limit checking on code or data segments.
func getLimit(entry uint64) uint32 {
	limit := uint32((((entry) & 0x000F000000000000) >> 32) | ((entry) & 0x000000000000FFFF))

	// Perform manual limit scaling if G flag is set
	if getG(entry) == 0 {
		return limit
	}

	// G flag is either 0 or 1. Scale out to 32-bit ceiling boundaries.
	return (limit << 12) | 0xFFF
}

func getG(entry uint64) uint8 {
	return uint8((entry & 0x0080000000000000) >> 55)
}

func getDb(entry uint64) uint8 {
	return uint8((entry & 0x0040000000000000) >> 54)
}

func getL(entry uint64) uint8 {
	return uint8((entry & 0x0020000000000000) >> 53)
}

func getAvl(entry uint64) uint8 {
	return uint8((entry & 0x0010000000000000) >> 52)
}

func getP(entry uint64) uint8 {
	return uint8((entry & 0x0000800000000000) >> 47)
}

func getDpl(entry uint64) uint8 {
	return uint8((entry & 0x0000600000000000) >> 45)
}

func getS(entry uint64) uint8 {
	return uint8((entry & 0x0000100000000000) >> 44)
}

func getType(entry uint64) uint8 {
	return uint8((entry & 0x00000F0000000000) >> 40)
}

// SegmentFromGdt automatically builds the struct for SET_SREGS from the kernel bit fields.
func SegmentFromGdt(entry uint64, tableIndex uint8) hyper.SegmentRegister {
	present := getP(entry)
	var unusable uint8 = 0
	if present == 0 {
		unusable = 1
	}

	return hyper.SegmentRegister{
		Base:     getBase(entry),
		Limit:    getLimit(entry),
		Selector: uint16(tableIndex) * 8,
		Type:     getType(entry),
		Present:  present,
		Dpl:      getDpl(entry),
		Db:       getDb(entry),
		S:        getS(entry),
		L:        getL(entry),
		G:        getG(entry),
		Avl:      getAvl(entry),
		Unusable: unusable,
	}
}
