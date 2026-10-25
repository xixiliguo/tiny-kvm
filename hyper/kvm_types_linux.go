//go:build ignore

/*
Input to cgo -godefs.
*/

package hyper

/*
#include <linux/kvm.h>



// KVM_EXIT_UNKNOWN
struct hw{
	__u64 hardware_exit_reason;
};

// KVM_EXIT_FAIL_ENTRY
struct fail_entry{
	__u64 hardware_entry_failure_reason;
	__u32 cpu;
};

// KVM_EXIT_EXCEPTION
struct ex{
	__u32 exception;
	__u32 error_code;
};

// KVM_EXIT_IO
struct io{
#define KVM_EXIT_IO_IN  0
#define KVM_EXIT_IO_OUT 1
	__u8 direction;
	__u8 size; // bytes
	__u16 port;
	__u32 count;
	__u64 data_offset; // relative to kvm_run start
};

// KVM_EXIT_DEBUG
struct debug{
	struct kvm_debug_exit_arch arch;
};

// KVM_EXIT_MMIO
struct mmio{
	__u64 phys_addr;
	__u8  data[8];
	__u32 len;
	__u8  is_write;
};

// KVM_EXIT_HYPERCALL
struct hypercall{
	__u64 nr;
	__u64 args[6];
	__u64 ret;

	union {
		__u32 longmode;
		__u64 flags;
	};
};

// KVM_EXIT_TPR_ACCESS
struct tpr_access{
	__u64 rip;
	__u32 is_write;
	__u32 pad;
};

// KVM_EXIT_S390_SIEIC
struct s390_sieic{
	__u8 icptcode;
	__u16 ipa;
	__u32 ipb;
};

// KVM_EXIT_S390_RESET
__u64 s390_reset_flags;

// KVM_EXIT_S390_UCONTROL
struct s390_ucontrol{
	__u64 trans_exc_code;
	__u32 pgm_code;
};

// KVM_EXIT_DCR (deprecated)
struct dcr{
	__u32 dcrn;
	__u32 data;
	__u8  is_write;
};

// KVM_EXIT_INTERNAL_ERROR
struct internal{
	__u32 suberror;
	// Available with KVM_CAP_INTERNAL_ERROR_DATA:
	__u32 ndata;
	__u64 data[16];
};

struct emulation_failure{
	__u32 suberror;
	__u32 ndata;
	__u64 flags;
	union {
		struct {
			__u8  insn_size;
			__u8  insn_bytes[15];
		};
	};
	// Arbitrary debug data may follow.
};

// KVM_EXIT_OSI
struct osi{
	__u64 gprs[32];
};

// KVM_EXIT_PAPR_HCALL
struct papr_hcall{
	__u64 nr;
	__u64 ret;
	__u64 args[9];
};

// KVM_EXIT_S390_TSCH
struct s390_tsch{
	__u16 subchannel_id;
	__u16 subchannel_nr;
	__u32 io_int_parm;
	__u32 io_int_word;
	__u32 ipb;
	__u8 dequeued;
};

// KVM_EXIT_EPR
struct epr{
	__u32 epr;
};

// KVM_EXIT_SYSTEM_EVENT
struct system_event{
#define KVM_SYSTEM_EVENT_SHUTDOWN       1
#define KVM_SYSTEM_EVENT_RESET          2
#define KVM_SYSTEM_EVENT_CRASH          3
#define KVM_SYSTEM_EVENT_WAKEUP         4
#define KVM_SYSTEM_EVENT_SUSPEND        5
#define KVM_SYSTEM_EVENT_SEV_TERM       6
	__u32 type;
	__u32 ndata;
	union {
		__u64 flags;
		__u64 data[16];
	};
};

// KVM_EXIT_S390_STSI
struct s390_stsi{
	__u64 addr;
	__u8 ar;
	__u8 reserved;
	__u8 fc;
	__u8 sel1;
	__u16 sel2;
};

// KVM_EXIT_IOAPIC_EOI
struct eoi{
	__u8 vector;
};

// KVM_EXIT_ARM_NISV
struct arm_nisv{
	__u64 esr_iss;
	__u64 fault_ipa;
};

// KVM_EXIT_X86_RDMSR / KVM_EXIT_X86_WRMSR
struct msr{
	__u8 error; // user -> kernel
	__u8 pad[7];
#define KVM_MSR_EXIT_REASON_INVAL	(1 << 0)
#define KVM_MSR_EXIT_REASON_UNKNOWN	(1 << 1)
#define KVM_MSR_EXIT_REASON_FILTER	(1 << 2)
#define KVM_MSR_EXIT_REASON_VALID_MASK	(KVM_MSR_EXIT_REASON_INVAL   |	\
				KVM_MSR_EXIT_REASON_UNKNOWN |	\
				KVM_MSR_EXIT_REASON_FILTER)
	__u32 reason; // kernel -> user
	__u32 index; // kernel -> user
	__u64 data; // kernel <-> user
};


// KVM_EXIT_NOTIFY
struct notify{
#define KVM_NOTIFY_CONTEXT_INVALID	(1 << 0)
	__u32 flags;
};

// KVM_EXIT_MEMORY_FAULT
struct memory_fault{
#define KVM_MEMORY_EXIT_FLAG_PRIVATE	(1ULL << 3)
	__u64 flags;
	__u64 gpa;
	__u64 size;
};

// for KVM_RUN, returned by mmap(vcpu_fd, offset=0)
struct my_kvm_run {
	// in
	__u8 request_interrupt_window;
	__u8 immediate_exit;
	__u8 padding1[6];

	// out
	__u32 exit_reason;
	__u8 ready_for_interrupt_injection;
	__u8 if_flag;
	__u16 flags;

	// in (pre_kvm_run), out (post_kvm_run)
	__u64 cr8;
	__u64 apic_base;

#ifdef __KVM_S390
	// the processor status word for s390
	__u64 psw_mask; // psw upper half
	__u64 psw_addr; // psw lower half
#endif
	union {
		// Fix the size of the union.
		char data[256];
	};


	#define SYNC_REGS_SIZE_BYTES 2048

	__u64 kvm_valid_regs;
	__u64 kvm_dirty_regs;
	union {
		struct kvm_sync_regs regs;
		char padding[SYNC_REGS_SIZE_BYTES];
	} s;
};
*/
import "C"

const (
	KVM_API_VERSION = C.KVM_API_VERSION
)

type KvmUserspaceMemoryRegion C.struct_kvm_userspace_memory_region

type KvmIoapicState C.struct_kvm_ioapic_state
type KvmPicState C.struct_kvm_pic_state
type KvmIrqchip C.struct_kvm_irqchip

type KvmPitConfig C.struct_kvm_pit_config

type KvmExitReason uint32

const (
	KvmExitUnkonw        KvmExitReason = C.KVM_EXIT_UNKNOWN
	KvmExitException     KvmExitReason = C.KVM_EXIT_EXCEPTION
	KvmExitIO            KvmExitReason = C.KVM_EXIT_IO
	KvmExitHypercall     KvmExitReason = C.KVM_EXIT_HYPERCALL
	KvmExitDebug         KvmExitReason = C.KVM_EXIT_DEBUG
	KvmExitHlt           KvmExitReason = C.KVM_EXIT_HLT
	KvmExitMmio          KvmExitReason = C.KVM_EXIT_MMIO
	KvmExitIrqWinOpen    KvmExitReason = C.KVM_EXIT_IRQ_WINDOW_OPEN
	KvmExitShutDown      KvmExitReason = C.KVM_EXIT_SHUTDOWN
	KvmExitFailEntry     KvmExitReason = C.KVM_EXIT_FAIL_ENTRY
	KvmExitIntr          KvmExitReason = C.KVM_EXIT_INTR
	KvmExitSetTpr        KvmExitReason = C.KVM_EXIT_SET_TPR
	KvmExitTprAccess     KvmExitReason = C.KVM_EXIT_TPR_ACCESS
	KvmExitS390Sieic     KvmExitReason = C.KVM_EXIT_S390_SIEIC
	KvmExitS390Reset     KvmExitReason = C.KVM_EXIT_S390_RESET
	KvmExitDcr           KvmExitReason = C.KVM_EXIT_DCR
	KvmExitNmi           KvmExitReason = C.KVM_EXIT_NMI
	KvmExitInternalError KvmExitReason = C.KVM_EXIT_INTERNAL_ERROR
	KvmExitOsi           KvmExitReason = C.KVM_EXIT_OSI
	KvmExitPaprHcall     KvmExitReason = C.KVM_EXIT_PAPR_HCALL
	KvmExitS390Ucontrol  KvmExitReason = C.KVM_EXIT_S390_UCONTROL
	KvmExitWatchdog      KvmExitReason = C.KVM_EXIT_WATCHDOG
	KvmExitS390Tsch      KvmExitReason = C.KVM_EXIT_S390_TSCH
	KvmExitEpr           KvmExitReason = C.KVM_EXIT_EPR
	KvmExitSystemEvent   KvmExitReason = C.KVM_EXIT_SYSTEM_EVENT
	KvmExitS390Stsi      KvmExitReason = C.KVM_EXIT_S390_STSI
	KvmExitIoapciEoi     KvmExitReason = C.KVM_EXIT_IOAPIC_EOI
	KvmExitHyperv        KvmExitReason = C.KVM_EXIT_HYPERV
	KvmExitArmNisv       KvmExitReason = C.KVM_EXIT_ARM_NISV
	KvmExitX86Rdmsr      KvmExitReason = C.KVM_EXIT_X86_RDMSR
	KvmExitX86Wrmsr      KvmExitReason = C.KVM_EXIT_X86_WRMSR
	KvmExitDirtyRingFull KvmExitReason = C.KVM_EXIT_DIRTY_RING_FULL
	KvmExitApResetHold   KvmExitReason = C.KVM_EXIT_AP_RESET_HOLD
	KvmExitX86BusLock    KvmExitReason = C.KVM_EXIT_X86_BUS_LOCK
	KvmExitXen           KvmExitReason = C.KVM_EXIT_XEN
	KvmExitNotify        KvmExitReason = C.KVM_EXIT_NOTIFY
	KvmExitMemoryFault   KvmExitReason = C.KVM_EXIT_MEMORY_FAULT
)

type hw C.struct_hw
type fail_entry C.struct_fail_entry
type ex C.struct_ex
type io C.struct_io

const (
	KVM_EXIT_IO_IN  = C.KVM_EXIT_IO_IN
	KVM_EXIT_IO_OUT = C.KVM_EXIT_IO_OUT
)

type kvm_debug_exit_arch C.struct_kvm_debug_exit_arch
type debug C.struct_debug
type mmio C.struct_mmio
type hypercall C.struct_hypercall
type msr C.struct_msr

// flag for vmexit msr:
const (
	KVM_MSR_EXIT_REASON_INVAL      = C.KVM_MSR_EXIT_REASON_INVAL
	KVM_MSR_EXIT_REASON_UNKNOWN    = C.KVM_MSR_EXIT_REASON_UNKNOWN
	KVM_MSR_EXIT_REASON_FILTER     = C.KVM_MSR_EXIT_REASON_FILTER
	KVM_MSR_EXIT_REASON_VALID_MASK = C.KVM_MSR_EXIT_REASON_VALID_MASK
)

type eoi C.struct_eoi

type tpr_access C.struct_tpr_access

type KvmRun C.struct_my_kvm_run

const (
	KVM_IOEVENTFD_FLAG_DATAMATCH         = C.KVM_IOEVENTFD_FLAG_DATAMATCH
	KVM_IOEVENTFD_FLAG_PIO               = C.KVM_IOEVENTFD_FLAG_PIO
	KVM_IOEVENTFD_FLAG_DEASSIGN          = C.KVM_IOEVENTFD_FLAG_DEASSIGN
	KVM_IOEVENTFD_FLAG_VIRTIO_CCW_NOTIFY = C.KVM_IOEVENTFD_FLAG_VIRTIO_CCW_NOTIFY
	KVM_IOEVENTFD_VALID_FLAG_MASK        = C.KVM_IOEVENTFD_VALID_FLAG_MASK
)

type KvmIoEventFd C.struct_kvm_ioeventfd

type KvmEnableCap C.struct_kvm_enable_cap

// ioctls for /dev/kvm fds:
const (
	KVM_GET_API_VERSION            = C.KVM_GET_API_VERSION
	KVM_CREATE_VM                  = C.KVM_CREATE_VM
	KVM_GET_MSR_INDEX_LIST         = C.KVM_GET_MSR_INDEX_LIST
	KVM_S390_ENABLE_SIE            = C.KVM_S390_ENABLE_SIE
	KVM_CHECK_EXTENSION            = C.KVM_CHECK_EXTENSION
	KVM_GET_VCPU_MMAP_SIZE         = C.KVM_GET_VCPU_MMAP_SIZE
	KVM_GET_SUPPORTED_CPUID        = C.KVM_GET_SUPPORTED_CPUID
	KVM_GET_EMULATED_CPUID         = C.KVM_GET_EMULATED_CPUID
	KVM_GET_MSR_FEATURE_INDEX_LIST = C.KVM_GET_MSR_FEATURE_INDEX_LIST

	KVM_CREATE_VCPU             = C.KVM_CREATE_VCPU
	KVM_GET_DIRTY_LOG           = C.KVM_GET_DIRTY_LOG
	KVM_SET_NR_MMU_PAGES        = C.KVM_SET_NR_MMU_PAGES
	KVM_GET_NR_MMU_PAGES        = C.KVM_GET_NR_MMU_PAGES /* deprecated */
	KVM_SET_USER_MEMORY_REGION  = C.KVM_SET_USER_MEMORY_REGION
	KVM_SET_TSS_ADDR            = C.KVM_SET_TSS_ADDR
	KVM_SET_IDENTITY_MAP_ADDR   = C.KVM_SET_IDENTITY_MAP_ADDR
	KVM_SET_USER_MEMORY_REGION2 = C.KVM_SET_USER_MEMORY_REGION2

	/* Device model IOC */
	KVM_CREATE_IRQCHIP            = C.KVM_CREATE_IRQCHIP
	KVM_IRQ_LINE                  = C.KVM_IRQ_LINE
	KVM_GET_IRQCHIP               = C.KVM_GET_IRQCHIP
	KVM_SET_IRQCHIP               = C.KVM_SET_IRQCHIP
	KVM_CREATE_PIT                = C.KVM_CREATE_PIT
	KVM_GET_PIT                   = C.KVM_GET_PIT
	KVM_SET_PIT                   = C.KVM_SET_PIT
	KVM_IRQ_LINE_STATUS           = C.KVM_IRQ_LINE_STATUS
	KVM_REGISTER_COALESCED_MMIO   = C.KVM_REGISTER_COALESCED_MMIO
	KVM_UNREGISTER_COALESCED_MMIO = C.KVM_UNREGISTER_COALESCED_MMIO
	KVM_SET_GSI_ROUTING           = C.KVM_SET_GSI_ROUTING
	KVM_REINJECT_CONTROL          = C.KVM_REINJECT_CONTROL
	KVM_IRQFD                     = C.KVM_IRQFD
	KVM_CREATE_PIT2               = C.KVM_CREATE_PIT2
	KVM_SET_BOOT_CPU_ID           = C.KVM_SET_BOOT_CPU_ID
	KVM_IOEVENTFD                 = C.KVM_IOEVENTFD
	KVM_XEN_HVM_CONFIG            = C.KVM_XEN_HVM_CONFIG
	KVM_SET_CLOCK                 = C.KVM_SET_CLOCK
	KVM_GET_CLOCK                 = C.KVM_GET_CLOCK

	/* Available with KVM_CAP_PIT_STATE2 */
	KVM_GET_PIT2 = C.KVM_GET_PIT2
	KVM_SET_PIT2 = C.KVM_SET_PIT2

	/* Available with KVM_CAP_TSC_CONTROL for a vCPU, or with
	* KVM_CAP_VM_TSC_CONTROL to set defaults for a VM */
	KVM_SET_TSC_KHZ = C.KVM_SET_TSC_KHZ
	KVM_GET_TSC_KHZ = C.KVM_GET_TSC_KHZ

	/* Available with KVM_CAP_SIGNAL_MSI */
	KVM_SIGNAL_MSI = C.KVM_SIGNAL_MSI

	/* Available with KVM_CAP_PPC_ALLOC_HTAB */
	KVM_PPC_ALLOCATE_HTAB = C.KVM_PPC_ALLOCATE_HTAB

	/* Available with KVM_CAP_ARM_SET_DEVICE_ADDR */
	KVM_ARM_SET_DEVICE_ADDR = C.KVM_ARM_SET_DEVICE_ADDR

	/* Available with KVM_CAP_PMU_EVENT_FILTER */
	KVM_SET_PMU_EVENT_FILTER = C.KVM_SET_PMU_EVENT_FILTER
	KVM_PPC_SVM_OFF          = C.KVM_PPC_SVM_OFF

	/* ioctl for vm fd */
	KVM_CREATE_DEVICE = C.KVM_CREATE_DEVICE

	/* ioctls for fds returned by KVM_CREATE_DEVICE */
	KVM_SET_DEVICE_ATTR = C.KVM_SET_DEVICE_ATTR
	KVM_GET_DEVICE_ATTR = C.KVM_GET_DEVICE_ATTR
	KVM_HAS_DEVICE_ATTR = C.KVM_HAS_DEVICE_ATTR
)

type KvmCap int

// Extension capability list.
const (
	KvmCapIrqchip                   KvmCap = C.KVM_CAP_IRQCHIP
	KvmCapHlt                       KvmCap = C.KVM_CAP_HLT
	KvmCapMmuShadowCacheControl     KvmCap = C.KVM_CAP_MMU_SHADOW_CACHE_CONTROL
	KvmCapUserMemory                KvmCap = C.KVM_CAP_USER_MEMORY
	KvmCapSetTssAddr                KvmCap = C.KVM_CAP_SET_TSS_ADDR
	KvmCapVapic                     KvmCap = C.KVM_CAP_VAPIC
	KvmCapExtCpuids                 KvmCap = C.KVM_CAP_EXT_CPUID
	KvmCapClocksource               KvmCap = C.KVM_CAP_CLOCKSOURCE
	KvmCapNrVcpus                   KvmCap = C.KVM_CAP_NR_VCPUS    /* returns recommended max vcpus per vm */
	KvmCapNrMemslots                KvmCap = C.KVM_CAP_NR_MEMSLOTS /* returns max memory slots per vm */
	KvmCapPit                       KvmCap = C.KVM_CAP_PIT
	KvmCapNopIoDelay                KvmCap = C.KVM_CAP_NOP_IO_DELAY
	KvmCapPvMmu                     KvmCap = C.KVM_CAP_PV_MMU
	KvmCapMpState                   KvmCap = C.KVM_CAP_MP_STATE
	KvmCapCoalescedMmio             KvmCap = C.KVM_CAP_COALESCED_MMIO
	KvmCapSyncMmu                   KvmCap = C.KVM_CAP_SYNC_MMU /* Changes to host mmap are reflected in guest */
	KvmCapIommu                     KvmCap = C.KVM_CAP_IOMMU
	KvmCapDestroyMemoryRegionWorks  KvmCap = C.KVM_CAP_DESTROY_MEMORY_REGION_WORKS
	KvmCapUserNmi                   KvmCap = C.KVM_CAP_USER_NMI
	KvmCapSetGuestDebug             KvmCap = C.KVM_CAP_SET_GUEST_DEBUG
	KvmCapReinjectControl           KvmCap = C.KVM_CAP_REINJECT_CONTROL
	KvmCapIrqRouting                KvmCap = C.KVM_CAP_IRQ_ROUTING
	KvmCapIrqInjectStatus           KvmCap = C.KVM_CAP_IRQ_INJECT_STATUS
	KvmCapAssignDevIrq              KvmCap = C.KVM_CAP_ASSIGN_DEV_IRQ
	KvmCapJoinMemoryRegionsWorks    KvmCap = C.KVM_CAP_JOIN_MEMORY_REGIONS_WORKS
	KvmCapMce                       KvmCap = C.KVM_CAP_MCE
	KvmCapIrqfd                     KvmCap = C.KVM_CAP_IRQFD
	KvmCapPit2                      KvmCap = C.KVM_CAP_PIT2
	KvmCapSetBootCpuId              KvmCap = C.KVM_CAP_SET_BOOT_CPU_ID
	KvmCapPitState2                 KvmCap = C.KVM_CAP_PIT_STATE2
	KvmCapIoeventfd                 KvmCap = C.KVM_CAP_IOEVENTFD
	KvmCapSetIdentityMapAddr        KvmCap = C.KVM_CAP_SET_IDENTITY_MAP_ADDR
	KvmCapXenHvm                    KvmCap = C.KVM_CAP_XEN_HVM
	KvmCapAdjustClock               KvmCap = C.KVM_CAP_ADJUST_CLOCK
	KvmCapInternalErrorData         KvmCap = C.KVM_CAP_INTERNAL_ERROR_DATA
	KvmCapVcpuEvents                KvmCap = C.KVM_CAP_VCPU_EVENTS
	KvmCapS390Psw                   KvmCap = C.KVM_CAP_S390_PSW
	KvmCapPpcSegstate               KvmCap = C.KVM_CAP_PPC_SEGSTATE
	KvmCapHyperv                    KvmCap = C.KVM_CAP_HYPERV
	KvmCapHypervVapic               KvmCap = C.KVM_CAP_HYPERV_VAPIC
	KvmCapHypervSpin                KvmCap = C.KVM_CAP_HYPERV_SPIN
	KvmCapPciSegment                KvmCap = C.KVM_CAP_PCI_SEGMENT
	KvmCapPpcPairedSingles          KvmCap = C.KVM_CAP_PPC_PAIRED_SINGLES
	KvmCapIntrShadow                KvmCap = C.KVM_CAP_INTR_SHADOW
	KvmCapDebugregs                 KvmCap = C.KVM_CAP_DEBUGREGS
	KvmCapX86RobustSinglestep       KvmCap = C.KVM_CAP_X86_ROBUST_SINGLESTEP
	KvmCapPpcOsi                    KvmCap = C.KVM_CAP_PPC_OSI
	KvmCapPpcUnsetIrq               KvmCap = C.KVM_CAP_PPC_UNSET_IRQ
	KvmCapEnableCap                 KvmCap = C.KVM_CAP_ENABLE_CAP
	KvmCapXsave                     KvmCap = C.KVM_CAP_XSAVE
	KvmCapXcrs                      KvmCap = C.KVM_CAP_XCRS
	KvmCapPpcGetPvinfo              KvmCap = C.KVM_CAP_PPC_GET_PVINFO
	KvmCapPpcIrqLevel               KvmCap = C.KVM_CAP_PPC_IRQ_LEVEL
	KvmCapAsyncPf                   KvmCap = C.KVM_CAP_ASYNC_PF
	KvmCapTscControl                KvmCap = C.KVM_CAP_TSC_CONTROL
	KvmCapGetTscKhz                 KvmCap = C.KVM_CAP_GET_TSC_KHZ
	KvmCapPpcBookeSregs             KvmCap = C.KVM_CAP_PPC_BOOKE_SREGS
	KvmCapSpaprTce                  KvmCap = C.KVM_CAP_SPAPR_TCE
	KvmCapPpcSmt                    KvmCap = C.KVM_CAP_PPC_SMT
	KvmCapPpcRma                    KvmCap = C.KVM_CAP_PPC_RMA
	KvmCapMaxVcpus                  KvmCap = C.KVM_CAP_MAX_VCPUS /* returns max vcpus per vm */
	KvmCapPpcHior                   KvmCap = C.KVM_CAP_PPC_HIOR
	KvmCapPpcPapr                   KvmCap = C.KVM_CAP_PPC_PAPR
	KvmCapSwTlb                     KvmCap = C.KVM_CAP_SW_TLB
	KvmCapOneReg                    KvmCap = C.KVM_CAP_ONE_REG
	KvmCapS390Gmap                  KvmCap = C.KVM_CAP_S390_GMAP
	KvmCapTscDeadlineTimer          KvmCap = C.KVM_CAP_TSC_DEADLINE_TIMER
	KvmCapS390Ucontrol              KvmCap = C.KVM_CAP_S390_UCONTROL
	KvmCapSyncRegs                  KvmCap = C.KVM_CAP_SYNC_REGS
	KvmCapPci23                     KvmCap = C.KVM_CAP_PCI_2_3
	KvmCapKvmclockCtrl              KvmCap = C.KVM_CAP_KVMCLOCK_CTRL
	KvmCapSignalMsi                 KvmCap = C.KVM_CAP_SIGNAL_MSI
	KvmCapPpcGetSmmuInfo            KvmCap = C.KVM_CAP_PPC_GET_SMMU_INFO
	KvmCapS390Cow                   KvmCap = C.KVM_CAP_S390_COW
	KvmCapPpcAllocHtab              KvmCap = C.KVM_CAP_PPC_ALLOC_HTAB
	KvmCapReadonlyMem               KvmCap = C.KVM_CAP_READONLY_MEM
	KvmCapIrqfdResample             KvmCap = C.KVM_CAP_IRQFD_RESAMPLE
	KvmCapPpcBookeWatchdog          KvmCap = C.KVM_CAP_PPC_BOOKE_WATCHDOG
	KvmCapPpcHtabFd                 KvmCap = C.KVM_CAP_PPC_HTAB_FD
	KvmCapS390CssSupport            KvmCap = C.KVM_CAP_S390_CSS_SUPPORT
	KvmCapPpcEpr                    KvmCap = C.KVM_CAP_PPC_EPR
	KvmCapArmPsci                   KvmCap = C.KVM_CAP_ARM_PSCI
	KvmCapArmSetDeviceAddr          KvmCap = C.KVM_CAP_ARM_SET_DEVICE_ADDR
	KvmCapDeviceCtrl                KvmCap = C.KVM_CAP_DEVICE_CTRL
	KvmCapIrqMpic                   KvmCap = C.KVM_CAP_IRQ_MPIC
	KvmCapPpcRtas                   KvmCap = C.KVM_CAP_PPC_RTAS
	KvmCapIrqXics                   KvmCap = C.KVM_CAP_IRQ_XICS
	KvmCapArmEl132bit               KvmCap = C.KVM_CAP_ARM_EL1_32BIT
	KvmCapSpaprMultitce             KvmCap = C.KVM_CAP_SPAPR_MULTITCE
	KvmCapExtEmulCpuid              KvmCap = C.KVM_CAP_EXT_EMUL_CPUID
	KvmCapHypervTime                KvmCap = C.KVM_CAP_HYPERV_TIME
	KvmCapIoapicPolarityIgnored     KvmCap = C.KVM_CAP_IOAPIC_POLARITY_IGNORED
	KvmCapEnableCapVm               KvmCap = C.KVM_CAP_ENABLE_CAP_VM
	KvmCapS390Irqchip               KvmCap = C.KVM_CAP_S390_IRQCHIP
	KvmCapIoeventfdNoLength         KvmCap = C.KVM_CAP_IOEVENTFD_NO_LENGTH
	KvmCapVmAttributes              KvmCap = C.KVM_CAP_VM_ATTRIBUTES
	KvmCapArmPsci02                 KvmCap = C.KVM_CAP_ARM_PSCI_0_2
	KvmCapPpcFixupHcall             KvmCap = C.KVM_CAP_PPC_FIXUP_HCALL
	KvmCapPpcEnableHcall            KvmCap = C.KVM_CAP_PPC_ENABLE_HCALL
	KvmCapCheckExtensionVm          KvmCap = C.KVM_CAP_CHECK_EXTENSION_VM
	KvmCapS390UserSigp              KvmCap = C.KVM_CAP_S390_USER_SIGP
	KvmCapS390VectorRegisters       KvmCap = C.KVM_CAP_S390_VECTOR_REGISTERS
	KvmCapS390MemOp                 KvmCap = C.KVM_CAP_S390_MEM_OP
	KvmCapS390UserStsi              KvmCap = C.KVM_CAP_S390_USER_STSI
	KvmCapS390Skeys                 KvmCap = C.KVM_CAP_S390_SKEYS
	KvmCapMipsFpu                   KvmCap = C.KVM_CAP_MIPS_FPU
	KvmCapMipsMsa                   KvmCap = C.KVM_CAP_MIPS_MSA
	KvmCapS390InjectIrq             KvmCap = C.KVM_CAP_S390_INJECT_IRQ
	KvmCapS390IrqState              KvmCap = C.KVM_CAP_S390_IRQ_STATE
	KvmCapPpcHwrng                  KvmCap = C.KVM_CAP_PPC_HWRNG
	KvmCapDisableQuirks             KvmCap = C.KVM_CAP_DISABLE_QUIRKS
	KvmCapX86Smm                    KvmCap = C.KVM_CAP_X86_SMM
	KvmCapMultiAddressSpace         KvmCap = C.KVM_CAP_MULTI_ADDRESS_SPACE
	KvmCapGuestDebugHwBps           KvmCap = C.KVM_CAP_GUEST_DEBUG_HW_BPS
	KvmCapGuestDebugHwWps           KvmCap = C.KVM_CAP_GUEST_DEBUG_HW_WPS
	KvmCapSplitIrqchip              KvmCap = C.KVM_CAP_SPLIT_IRQCHIP
	KvmCapIoeventfdAnyLength        KvmCap = C.KVM_CAP_IOEVENTFD_ANY_LENGTH
	KvmCapHypervSynic               KvmCap = C.KVM_CAP_HYPERV_SYNIC
	KvmCapS390Ri                    KvmCap = C.KVM_CAP_S390_RI
	KvmCapSpaprTce64                KvmCap = C.KVM_CAP_SPAPR_TCE_64
	KvmCapArmPmuV3                  KvmCap = C.KVM_CAP_ARM_PMU_V3
	KvmCapVcpuAttributes            KvmCap = C.KVM_CAP_VCPU_ATTRIBUTES
	KvmCapMaxVcpuId                 KvmCap = C.KVM_CAP_MAX_VCPU_ID
	KvmCapX2apicApi                 KvmCap = C.KVM_CAP_X2APIC_API
	KvmCapS390UserInstr0            KvmCap = C.KVM_CAP_S390_USER_INSTR0
	KvmCapMsiDevid                  KvmCap = C.KVM_CAP_MSI_DEVID
	KvmCapPpcHtm                    KvmCap = C.KVM_CAP_PPC_HTM
	KvmCapSpaprResizeHpt            KvmCap = C.KVM_CAP_SPAPR_RESIZE_HPT
	KvmCapPpcMmuRadix               KvmCap = C.KVM_CAP_PPC_MMU_RADIX
	KvmCapPpcMmuHashV3              KvmCap = C.KVM_CAP_PPC_MMU_HASH_V3
	KvmCapImmediateExit             KvmCap = C.KVM_CAP_IMMEDIATE_EXIT
	KvmCapMipsVz                    KvmCap = C.KVM_CAP_MIPS_VZ
	KvmCapMipsTe                    KvmCap = C.KVM_CAP_MIPS_TE
	KvmCapMips64bit                 KvmCap = C.KVM_CAP_MIPS_64BIT
	KvmCapS390Gs                    KvmCap = C.KVM_CAP_S390_GS
	KvmCapS390Ais                   KvmCap = C.KVM_CAP_S390_AIS
	KvmCapSpaprTceVfio              KvmCap = C.KVM_CAP_SPAPR_TCE_VFIO
	KvmCapX86DisableExits           KvmCap = C.KVM_CAP_X86_DISABLE_EXITS
	KvmCapArmUserIrq                KvmCap = C.KVM_CAP_ARM_USER_IRQ
	KvmCapS390CmmaMigration         KvmCap = C.KVM_CAP_S390_CMMA_MIGRATION
	KvmCapPpcFwnmi                  KvmCap = C.KVM_CAP_PPC_FWNMI
	KvmCapPpcSmtPossible            KvmCap = C.KVM_CAP_PPC_SMT_POSSIBLE
	KvmCapHypervSynic2              KvmCap = C.KVM_CAP_HYPERV_SYNIC2
	KvmCapHypervVpIndex             KvmCap = C.KVM_CAP_HYPERV_VP_INDEX
	KvmCapS390AisMigration          KvmCap = C.KVM_CAP_S390_AIS_MIGRATION
	KvmCapPpcGetCpuChar             KvmCap = C.KVM_CAP_PPC_GET_CPU_CHAR
	KvmCapS390Bpb                   KvmCap = C.KVM_CAP_S390_BPB
	KvmCapGetMsrFeatures            KvmCap = C.KVM_CAP_GET_MSR_FEATURES
	KvmCapHypervEventfd             KvmCap = C.KVM_CAP_HYPERV_EVENTFD
	KvmCapHypervTlbflush            KvmCap = C.KVM_CAP_HYPERV_TLBFLUSH
	KvmCapS390Hpage1m               KvmCap = C.KVM_CAP_S390_HPAGE_1M
	KvmCapNestedState               KvmCap = C.KVM_CAP_NESTED_STATE
	KvmCapArmInjectSerrorEsr        KvmCap = C.KVM_CAP_ARM_INJECT_SERROR_ESR
	KvmCapMsrPlatformInfo           KvmCap = C.KVM_CAP_MSR_PLATFORM_INFO
	KvmCapPpcNestedHv               KvmCap = C.KVM_CAP_PPC_NESTED_HV
	KvmCapHypervSendIpi             KvmCap = C.KVM_CAP_HYPERV_SEND_IPI
	KvmCapCoalescedPio              KvmCap = C.KVM_CAP_COALESCED_PIO
	KvmCapHypervEnlightenedVmcs     KvmCap = C.KVM_CAP_HYPERV_ENLIGHTENED_VMCS
	KvmCapExceptionPayload          KvmCap = C.KVM_CAP_EXCEPTION_PAYLOAD
	KvmCapArmVmIpaSize              KvmCap = C.KVM_CAP_ARM_VM_IPA_SIZE
	KvmCapManualDirtyLogProtect     KvmCap = C.KVM_CAP_MANUAL_DIRTY_LOG_PROTECT /* Obsolete */
	KvmCapHypervCpuid               KvmCap = C.KVM_CAP_HYPERV_CPUID
	KvmCapManualDirtyLogProtect2    KvmCap = C.KVM_CAP_MANUAL_DIRTY_LOG_PROTECT2
	KvmCapPpcIrqXive                KvmCap = C.KVM_CAP_PPC_IRQ_XIVE
	KvmCapArmSve                    KvmCap = C.KVM_CAP_ARM_SVE
	KvmCapArmPtrauthAddress         KvmCap = C.KVM_CAP_ARM_PTRAUTH_ADDRESS
	KvmCapArmPtrauthGeneric         KvmCap = C.KVM_CAP_ARM_PTRAUTH_GENERIC
	KvmCapPmuEventFilter            KvmCap = C.KVM_CAP_PMU_EVENT_FILTER
	KvmCapArmIrqLineLayout2         KvmCap = C.KVM_CAP_ARM_IRQ_LINE_LAYOUT_2
	KvmCapHypervDirectTlbflush      KvmCap = C.KVM_CAP_HYPERV_DIRECT_TLBFLUSH
	KvmCapPpcGuestDebugSstep        KvmCap = C.KVM_CAP_PPC_GUEST_DEBUG_SSTEP
	KvmCapArmNisvToUser             KvmCap = C.KVM_CAP_ARM_NISV_TO_USER
	KvmCapArmInjectExtDabt          KvmCap = C.KVM_CAP_ARM_INJECT_EXT_DABT
	KvmCapS390VcpuResets            KvmCap = C.KVM_CAP_S390_VCPU_RESETS
	KvmCapS390Protected             KvmCap = C.KVM_CAP_S390_PROTECTED
	KvmCapPpcSecureGuest            KvmCap = C.KVM_CAP_PPC_SECURE_GUEST
	KvmCapHaltPoll                  KvmCap = C.KVM_CAP_HALT_POLL
	KvmCapAsyncPfInt                KvmCap = C.KVM_CAP_ASYNC_PF_INT
	KvmCapLastCpu                   KvmCap = C.KVM_CAP_LAST_CPU
	KvmCapSmallerMaxphyaddr         KvmCap = C.KVM_CAP_SMALLER_MAXPHYADDR
	KvmCapS390Diag318               KvmCap = C.KVM_CAP_S390_DIAG318
	KvmCapStealTime                 KvmCap = C.KVM_CAP_STEAL_TIME
	KvmCapX86UserSpaceMsr           KvmCap = C.KVM_CAP_X86_USER_SPACE_MSR
	KvmCapX86MsrFilter              KvmCap = C.KVM_CAP_X86_MSR_FILTER
	KvmCapEnforcePvFeatureCpuid     KvmCap = C.KVM_CAP_ENFORCE_PV_FEATURE_CPUID
	KvmCapSysHypervCpuid            KvmCap = C.KVM_CAP_SYS_HYPERV_CPUID
	KvmCapDirtyLogRing              KvmCap = C.KVM_CAP_DIRTY_LOG_RING
	KvmCapX86BusLockExit            KvmCap = C.KVM_CAP_X86_BUS_LOCK_EXIT
	KvmCapPpcDawr1                  KvmCap = C.KVM_CAP_PPC_DAWR1
	KvmCapSetGuestDebug2            KvmCap = C.KVM_CAP_SET_GUEST_DEBUG2
	KvmCapSgxAttribute              KvmCap = C.KVM_CAP_SGX_ATTRIBUTE
	KvmCapVmCopyEncContextFrom      KvmCap = C.KVM_CAP_VM_COPY_ENC_CONTEXT_FROM
	KvmCapPtpKvm                    KvmCap = C.KVM_CAP_PTP_KVM
	KvmCapHypervEnforceCpuid        KvmCap = C.KVM_CAP_HYPERV_ENFORCE_CPUID
	KvmCapSregs2                    KvmCap = C.KVM_CAP_SREGS2
	KvmCapExitHypercall             KvmCap = C.KVM_CAP_EXIT_HYPERCALL
	KvmCapPpcRptInvalidate          KvmCap = C.KVM_CAP_PPC_RPT_INVALIDATE
	KvmCapBinaryStatsFd             KvmCap = C.KVM_CAP_BINARY_STATS_FD
	KvmCapExitOnEmulationFailure    KvmCap = C.KVM_CAP_EXIT_ON_EMULATION_FAILURE
	KvmCapArmMte                    KvmCap = C.KVM_CAP_ARM_MTE
	KvmCapVmMoveEncContextFrom      KvmCap = C.KVM_CAP_VM_MOVE_ENC_CONTEXT_FROM
	KvmCapXsave2                    KvmCap = C.KVM_CAP_XSAVE2
	KvmCapSysAttributes             KvmCap = C.KVM_CAP_SYS_ATTRIBUTES
	KvmCapPpcAilMode3               KvmCap = C.KVM_CAP_PPC_AIL_MODE_3
	KvmCapS390MemOpExtension        KvmCap = C.KVM_CAP_S390_MEM_OP_EXTENSION
	KvmCapPmuCapability             KvmCap = C.KVM_CAP_PMU_CAPABILITY
	KvmCapDisableQuirks2            KvmCap = C.KVM_CAP_DISABLE_QUIRKS2
	KvmCapVmTscControl              KvmCap = C.KVM_CAP_VM_TSC_CONTROL
	KvmCapSystemEventData           KvmCap = C.KVM_CAP_SYSTEM_EVENT_DATA
	KvmCapArmSystemSuspend          KvmCap = C.KVM_CAP_ARM_SYSTEM_SUSPEND
	KvmCapS390ProtectedDump         KvmCap = C.KVM_CAP_S390_PROTECTED_DUMP
	KvmCapX86TripleFaultEvent       KvmCap = C.KVM_CAP_X86_TRIPLE_FAULT_EVENT
	KvmCapX86NotifyVmexit           KvmCap = C.KVM_CAP_X86_NOTIFY_VMEXIT
	KvmCapVmDisableNxHugePages      KvmCap = C.KVM_CAP_VM_DISABLE_NX_HUGE_PAGES
	KvmCapS390ZpciOp                KvmCap = C.KVM_CAP_S390_ZPCI_OP
	KvmCapS390CpuTopology           KvmCap = C.KVM_CAP_S390_CPU_TOPOLOGY
	KvmCapDirtyLogRingAcqRel        KvmCap = C.KVM_CAP_DIRTY_LOG_RING_ACQ_REL
	KvmCapS390ProtectedAsyncDisable KvmCap = C.KVM_CAP_S390_PROTECTED_ASYNC_DISABLE
	KvmCapDirtyLogRingWithBitmap    KvmCap = C.KVM_CAP_DIRTY_LOG_RING_WITH_BITMAP
	KvmCapPmuEventMaskedEvents      KvmCap = C.KVM_CAP_PMU_EVENT_MASKED_EVENTS
	KvmCapCounterOffset             KvmCap = C.KVM_CAP_COUNTER_OFFSET
	KvmCapArmEagerSplitChunkSize    KvmCap = C.KVM_CAP_ARM_EAGER_SPLIT_CHUNK_SIZE
	KvmCapArmSupportedBlockSizes    KvmCap = C.KVM_CAP_ARM_SUPPORTED_BLOCK_SIZES
	KvmCapArmSupportedRegMaskRanges KvmCap = C.KVM_CAP_ARM_SUPPORTED_REG_MASK_RANGES
	KvmCapUserMemory2               KvmCap = C.KVM_CAP_USER_MEMORY2
	KvmCapMemoryFaultInfo           KvmCap = C.KVM_CAP_MEMORY_FAULT_INFO
	KvmCapMemoryAttributes          KvmCap = C.KVM_CAP_MEMORY_ATTRIBUTES
	KvmCapGuestMemfd                KvmCap = C.KVM_CAP_GUEST_MEMFD
	KvmCapVmTypes                   KvmCap = C.KVM_CAP_VM_TYPES
)

const (
	KVM_X2APIC_API_USE_32BIT_IDS           = C.KVM_X2APIC_API_USE_32BIT_IDS
	KVM_X2APIC_API_DISABLE_BROADCAST_QUIRK = C.KVM_X2APIC_API_DISABLE_BROADCAST_QUIRK
)

type KvmIrqRoutingIrqchip C.struct_kvm_irq_routing_irqchip
type KvmIrqRoutingMsi C.struct_kvm_irq_routing_msi

const (
	KVM_IRQ_ROUTING_IRQCHIP = C.KVM_IRQ_ROUTING_IRQCHIP
	KVM_IRQ_ROUTING_MSI     = C.KVM_IRQ_ROUTING_MSI
)

// type KvmIrqRoutingEntry C.struct_kvm_irq_routing_entry
type kvm_irq_routing C.struct_kvm_irq_routing

const (
	KVM_IRQFD_FLAG_DEASSIGN = C.KVM_IRQFD_FLAG_DEASSIGN
	KVM_IRQFD_FLAG_RESAMPLE = C.KVM_IRQFD_FLAG_RESAMPLE
)

type KvmIrqfd C.struct_kvm_irqfd

const (
	/*
	 * ioctls for vcpu fds
	 */
	KVM_RUN             = C.KVM_RUN
	KVM_GET_REGS        = C.KVM_GET_REGS
	KVM_SET_REGS        = C.KVM_SET_REGS
	KVM_GET_SREGS       = C.KVM_GET_SREGS
	KVM_SET_SREGS       = C.KVM_SET_SREGS
	KVM_TRANSLATE       = C.KVM_TRANSLATE
	KVM_INTERRUPT       = C.KVM_INTERRUPT
	KVM_GET_MSRS        = C.KVM_GET_MSRS
	KVM_SET_MSRS        = C.KVM_SET_MSRS
	KVM_SET_CPUID       = C.KVM_SET_CPUID
	KVM_SET_SIGNAL_MASK = C.KVM_SET_SIGNAL_MASK
	KVM_GET_FPU         = C.KVM_GET_FPU
	KVM_SET_FPU         = C.KVM_SET_FPU
	KVM_GET_LAPIC       = C.KVM_GET_LAPIC
	KVM_SET_LAPIC       = C.KVM_SET_LAPIC
	KVM_SET_CPUID2      = C.KVM_SET_CPUID2
	KVM_GET_CPUID2      = C.KVM_GET_CPUID2

	/* Available with KVM_CAP_VAPIC */
	KVM_TPR_ACCESS_REPORTING = C.KVM_TPR_ACCESS_REPORTING
	/* Available with KVM_CAP_VAPIC */
	KVM_SET_VAPIC_ADDR = C.KVM_SET_VAPIC_ADDR

	/* store status for s390 */
	KVM_S390_STORE_STATUS_NOADDR   = ^uintptr(0)     // 对应 C 语言的 -1ul
	KVM_S390_STORE_STATUS_PREFIXED = ^uintptr(0) - 1 // 对应 C 语言的 -2ul
	KVM_S390_STORE_STATUS          = C.KVM_S390_STORE_STATUS

	/* initial reset for s390 */
	KVM_S390_INITIAL_RESET = C.KVM_S390_INITIAL_RESET
	KVM_GET_MP_STATE       = C.KVM_GET_MP_STATE
	KVM_SET_MP_STATE       = C.KVM_SET_MP_STATE

	/* Available with KVM_CAP_USER_NMI */
	KVM_NMI = C.KVM_NMI
	/* Available with KVM_CAP_SET_GUEST_DEBUG */
	KVM_SET_GUEST_DEBUG = C.KVM_SET_GUEST_DEBUG

	/* MCE for x86 */
	KVM_X86_SETUP_MCE             = C.KVM_X86_SETUP_MCE
	KVM_X86_GET_MCE_CAP_SUPPORTED = C.KVM_X86_GET_MCE_CAP_SUPPORTED
	KVM_X86_SET_MCE               = C.KVM_X86_SET_MCE

	/* Available with KVM_CAP_VCPU_EVENTS */
	KVM_GET_VCPU_EVENTS = C.KVM_GET_VCPU_EVENTS
	KVM_SET_VCPU_EVENTS = C.KVM_SET_VCPU_EVENTS
	/* Available with KVM_CAP_DEBUGREGS */
	KVM_GET_DEBUGREGS = C.KVM_GET_DEBUGREGS
	KVM_SET_DEBUGREGS = C.KVM_SET_DEBUGREGS

	/*
	 * vcpu version available with KVM_CAP_ENABLE_CAP
	 * vm version available with KVM_CAP_ENABLE_CAP_VM
	 */
	KVM_ENABLE_CAP = C.KVM_ENABLE_CAP
	/* Available with KVM_CAP_XSAVE */
	KVM_GET_XSAVE = C.KVM_GET_XSAVE
	KVM_SET_XSAVE = C.KVM_SET_XSAVE
	/* Available with KVM_CAP_XCRS */
	KVM_GET_XCRS = C.KVM_GET_XCRS
	KVM_SET_XCRS = C.KVM_SET_XCRS
	/* Available with KVM_CAP_SW_TLB */
	KVM_DIRTY_TLB = C.KVM_DIRTY_TLB
	/* Available with KVM_CAP_ONE_REG */
	KVM_GET_ONE_REG = C.KVM_GET_ONE_REG
	KVM_SET_ONE_REG = C.KVM_SET_ONE_REG

	/* VM is being stopped by host */
	KVM_KVMCLOCK_CTRL = C.KVM_KVMCLOCK_CTRL
	KVM_GET_REG_LIST  = C.KVM_GET_REG_LIST

	/* Available with KVM_CAP_X86_SMM */
	KVM_SMI = C.KVM_SMI

	/* Memory Encryption Commands */
	KVM_MEMORY_ENCRYPT_OP = C.KVM_MEMORY_ENCRYPT_OP

	KVM_MEMORY_ENCRYPT_REG_REGION   = C.KVM_MEMORY_ENCRYPT_REG_REGION
	KVM_MEMORY_ENCRYPT_UNREG_REGION = C.KVM_MEMORY_ENCRYPT_UNREG_REGION

	/* Available with KVM_CAP_HYPERV_EVENTFD */
	KVM_HYPERV_EVENTFD = C.KVM_HYPERV_EVENTFD

	/* Available with KVM_CAP_NESTED_STATE */
	KVM_GET_NESTED_STATE = C.KVM_GET_NESTED_STATE
	KVM_SET_NESTED_STATE = C.KVM_SET_NESTED_STATE

	/* Available with KVM_CAP_MANUAL_DIRTY_LOG_PROTECT_2 */
	KVM_CLEAR_DIRTY_LOG = C.KVM_CLEAR_DIRTY_LOG

	/* Available with KVM_CAP_HYPERV_CPUID (vcpu) / KVM_CAP_SYS_HYPERV_CPUID (system) */
	KVM_GET_SUPPORTED_HV_CPUID = C.KVM_GET_SUPPORTED_HV_CPUID

	/* Available with KVM_CAP_ARM_SVE */
	KVM_ARM_VCPU_FINALIZE = C.KVM_ARM_VCPU_FINALIZE

	/* Available with  KVM_CAP_S390_VCPU_RESETS */
	KVM_S390_NORMAL_RESET = C.KVM_S390_NORMAL_RESET
	KVM_S390_CLEAR_RESET  = C.KVM_S390_CLEAR_RESET

	/* Available with KVM_CAP_X86_MSR_FILTER */
	KVM_X86_SET_MSR_FILTER = C.KVM_X86_SET_MSR_FILTER

	/* Available with KVM_CAP_DIRTY_LOG_RING */
	KVM_RESET_DIRTY_RINGS = C.KVM_RESET_DIRTY_RINGS

	/* Per-VM Xen attributes */
	KVM_XEN_HVM_GET_ATTR = C.KVM_XEN_HVM_GET_ATTR
	KVM_XEN_HVM_SET_ATTR = C.KVM_XEN_HVM_SET_ATTR

	/* Per-vCPU Xen attributes */
	KVM_XEN_VCPU_GET_ATTR = C.KVM_XEN_VCPU_GET_ATTR
	KVM_XEN_VCPU_SET_ATTR = C.KVM_XEN_VCPU_SET_ATTR

	/* Available with KVM_CAP_XEN_HVM / KVM_XEN_HVM_CONFIG_EVTCHN_SEND */
	KVM_XEN_HVM_EVTCHN_SEND = C.KVM_XEN_HVM_EVTCHN_SEND

	KVM_GET_SREGS2 = C.KVM_GET_SREGS2
	KVM_SET_SREGS2 = C.KVM_SET_SREGS2

	KVM_GET_XSAVE2 = C.KVM_GET_XSAVE2
)

const (
	KVM_IOAPIC_NUM_PINS = C.KVM_IOAPIC_NUM_PINS
	KVM_APIC_REG_SIZE   = C.KVM_APIC_REG_SIZE
)

// for KVM_GET_REGS and KVM_SET_REGS
// type KvmRegs C.struct_kvm_regs

// for KVM_GET_LAPIC and KVM_SET_LAPIC
type KvmLapicState C.struct_kvm_lapic_state

// type KvmSegment C.struct_kvm_segment
// type KvmDtable C.struct_kvm_dtable

// for KVM_GET_SREGS and KVM_SET_SREGS
// type KvmSregs C.struct_kvm_sregs

// for KVM_GET_FPU and KVM_SET_FPU
type KvmFpu C.struct_kvm_fpu

type KvmMsrEntry C.struct_kvm_msr_entry

// for KVM_GET_MSRS and KVM_SET_MSRS
type KvmMsrs C.struct_kvm_msrs

// for KVM_GET_MSR_INDEX_LIST
type KvmMsrList C.struct_kvm_msr_list

// type KvmCpuidEntry C.struct_kvm_cpuid_entry

// for KVM_SET_CPUID
// type KvmCpuid C.struct_kvm_cpuid

type KvmCpuidEntry2 C.struct_kvm_cpuid_entry2

// for KVM_SET_CPUID2
type KvmCpuid2 C.struct_kvm_cpuid2
