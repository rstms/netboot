// fat filesystem functions

package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func fatCmd(imageFile string, offset uint64, command string, args ...string) (string, error) {
	imageArg := imageFile
	if offset != 0 {
		imageArg = fmt.Sprintf("%s@@%d", imageFile, offset)
	}
	cmd := exec.Command(command, append([]string{"-i", imageArg}, args...)...)
	log.Printf("executing: %s\n", cmd)
	var obuf bytes.Buffer
	var ebuf bytes.Buffer
	cmd.Stdout = &obuf
	cmd.Stderr = &ebuf
	err := cmd.Run()
	if err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			return "", Fatalf("subprocess '%s' exited %d stderr='%s'\n", cmd, e.ExitCode(), ebuf.String())
		default:
			return "", Fatal(err)
		}
	}
	output := obuf.String()
	if output != "" {
		log.Printf("stdout: %s\n", output)
	}
	return output, nil
}

func CreateEFIImage(dstImage, efiBin, autoexec string) error {
	log.Printf("CreateEFIImage(%s, %s, %s)\n", dstImage, efiBin, autoexec)
	_, err := fatCmd(dstImage, 0, "mformat", "-C", "-f", "1440", "-F")
	if err != nil {
		return Fatal(err)
	}
	_, err = fatCmd(dstImage, 0, "mmd", "::/EFI")
	if err != nil {
		return Fatal(err)
	}
	_, err = fatCmd(dstImage, 0, "mmd", "::/EFI/BOOT")
	if err != nil {
		return Fatal(err)
	}
	_, name := filepath.Split(efiBin)
	_, err = fatCmd(dstImage, 0, "mcopy", efiBin, "::/EFI/BOOT/"+name)
	if err != nil {
		return Fatal(err)
	}
	_, err = fatCmd(dstImage, 0, "mcopy", autoexec, "::/autoexec.ipxe")
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func IsFAT(imagePathname string) (bool, error) {
	info, err := fatCmd(imagePathname, 0, "minfo")
	if err != nil {
		return false, nil
	}
	log.Printf("FAT info: %s\n", info)
	return true, nil
}

const (
	SECTOR_SIZE                = 512
	MBR_SIGNATURE_POS          = 510
	MBR_SIGNATURE_LEN          = 2
	MBR_SIGNATURE              = 0xaa55
	PARTITION_TABLE_POS        = 446
	PARTITION_RECORD_SIZE      = 16
	PARTITION_FLAGS_POS        = 0
	PARTITION_CHS_START_POS    = 1
	PARTITION_CHS_START_LEN    = 3
	PARTITION_TYPE_POS         = 4
	PARTITION_TYPE_LEN         = 1
	PARTITION_CHS_END_POS      = 5
	PARTITION_CHS_END_LEN      = 3
	PARTITION_LBA_START_POS    = 8
	PARTITION_LBA_START_LEN    = 4
	PARTITION_SECTOR_COUNT_POS = 12
	PARTITION_SECTOR_COUNT_LEN = 4
)

type CHS struct {
	Cylinder uint16
	Head     uint16
	Sector   uint16
}

func decodeCHS(chs *CHS, buf []byte) {
	//log.Printf("CHS: %02x %02x %02x\n", buf[0], buf[1], buf[2])
	chs.Head = uint16(buf[0])
	chs.Sector = uint16(buf[1]) & 0b00111111
	chs.Cylinder = ((uint16(buf[1]) & 0b11000000) << 8) | uint16(buf[2])
	//log.Printf("return: %+v\n", chs)
}

type PartitionType struct {
	Name string
	FAT  bool
}

type Partition struct {
	Active      bool
	StartCHS    CHS
	TypeCode    byte
	Type        PartitionType
	EndCHS      CHS
	StartSector uint32
	SectorCount uint32
	Offset      uint64
}

var TypeTable = map[byte]PartitionType{
	0x00: {"Empty", false},
	0x01: {"FAT12", true},
	0x04: {"FAT16 <32M", true},
	0x05: {"Extended", false},
	0x06: {"FAT16", true},
	0x07: {"HPFS/NTFS/exFAT", false},
	0x0b: {"W95 FAT32", true},
	0x0c: {"W95 FAT32 (LBA)", true},
	0x0e: {"W95 FAT16 (LBA)", true},
	0x0f: {"W95 Extended (LBA)", false},
	0x39: {"Plan 9", true},
	0x82: {"Linux Swap", false},
	0x83: {"Linux", false},
	0x85: {"Linux extended", false},
	0x86: {"NTFS volume set", false},
	0x87: {"NTFS volume set", false},
	0x88: {"Linux plaintext", false},
	0x8e: {"Linux LVM", false},
	0xa5: {"FreeBSD", false},
	0xa6: {"OpenBSD", false},
	0xee: {"GPT", false},
	0xef: {"EFI (FAT12/16/32)", true},
	0xfb: {"VMware VMFS", false},
	0xfc: {"VMware VMKCORE", false},
}

func ReadMBRPartitions(fatImage string) ([]Partition, error) {
	file, err := os.Open(fatImage)
	if err != nil {
		return nil, Fatal(err)
	}
	defer file.Close()
	header := make([]byte, SECTOR_SIZE)
	count, err := file.Read(header)
	if err != nil {
		return nil, Fatal(err)
	}
	if count != SECTOR_SIZE {
		return nil, Fatalf("read count mismatch: expected %d, got %d", SECTOR_SIZE, count)
	}
	magic := binary.LittleEndian.Uint16(header[MBR_SIGNATURE_POS : MBR_SIGNATURE_POS+MBR_SIGNATURE_LEN])
	//log.Printf("image magic: %04.4x\n", magic)
	if magic != MBR_SIGNATURE {
		return nil, Fatalf("invalid MBR signature: %x", magic)
	}
	table := make([]Partition, 4)
	for i := 0; i < 4; i++ {
		base := PARTITION_TABLE_POS + PARTITION_RECORD_SIZE*i
		table[i].Active = (header[base+PARTITION_FLAGS_POS] & 0x80) != 0
		decodeCHS(&table[i].StartCHS, header[base+PARTITION_CHS_START_POS:base+PARTITION_CHS_START_POS+PARTITION_CHS_START_LEN])
		table[i].TypeCode = header[base+PARTITION_TYPE_POS]
		ptype, ok := TypeTable[table[i].TypeCode]
		if !ok {
			ptype = PartitionType{"Other", false}
		}
		table[i].Type = ptype
		decodeCHS(&table[i].EndCHS, header[base+PARTITION_CHS_END_POS:base+PARTITION_CHS_END_POS+PARTITION_CHS_END_LEN])
		table[i].StartSector = binary.LittleEndian.Uint32(header[base+PARTITION_LBA_START_POS : base+PARTITION_LBA_START_POS+PARTITION_LBA_START_LEN])
		table[i].SectorCount = binary.LittleEndian.Uint32(header[base+PARTITION_SECTOR_COUNT_POS : base+PARTITION_SECTOR_COUNT_POS+PARTITION_SECTOR_COUNT_LEN])
		table[i].Offset = SECTOR_SIZE * uint64(table[i].StartSector)
		//log.Printf("partition=%d flags=%x chsStart=%+v typeCode=%x chsEnd=%+v lbaStart=%d sectorCount=%d\n", i, table[i].Flags, table[i].StartCHS, table[i].Type, table[i].EndCHS, table[i].StartSector, table[i].SectorCount)
	}
	return table, nil
}

func activePartitionOffset(fatImage string) (uint64, error) {
	fat, err := IsFAT(fatImage)
	if err != nil {
		return 0, Fatal(err)
	}
	if fat {
		return 0, nil
	}
	partitions, err := ReadMBRPartitions(fatImage)
	if err != nil {
		return 0, Fatal(err)
	}
	for i, partition := range partitions {
		if partition.Active && partition.Type.FAT {
			log.Printf("MBR Primary partition %d is type %02x (%s)", i, partition.TypeCode, partition.Type.Name)
			return partition.Offset, nil
		}
	}
	return 0, Fatalf("No active FAT partition")

}

func InjectBootFiles(dstImage string, bootFiles []string) error {
	log.Printf("injectBootFiles: %s %v\n", dstImage, bootFiles)
	offset, err := activePartitionOffset(dstImage)
	if err != nil {
		return Fatal(err)
	}
	for i, injectPathname := range bootFiles {
		_, name := filepath.Split(injectPathname)
		log.Printf("[%d] name=%s injectPath=%s\n", i, name, injectPathname)
		switch name {
		case "autoexec.ipxe.iso":
			name = ""
		case "autoexec.ipxe.img":
			name = "autoexec.ipxe"
		}
		if name != "" {
			log.Printf("injectBootFile %s -> %s\n", injectPathname, name)
			_, err := fatCmd(dstImage, offset, "mcopy", "-v", "-n", "-o", injectPathname, "::"+name)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	return nil
}
