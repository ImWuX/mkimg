package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/Shopify/go-lua"
	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

const MaxBootsectorSize = 440

func Ceil(value uint64, to uint64) uint64 {
	return (value + to - 1) / to * to
}

type Configuration struct {
	name        string
	dest        string
	pmbr        bool
	sectorSize  uint64
	firstSector uint64
	partitions  []Partition
	bootsector  string
}

type Partition struct {
	name     string
	typeUUID string
	ops      PartitionOperations
}

type PartitionOperations interface {
	size() uint64
	write(img *disk.Disk, index int)
}

type RawPartitionOperations struct {
	file *os.File
}

type FSPartitionOperations struct {
	partsize uint64
	fsType   filesystem.Type
	contents []FSPartitionContent
}

type FSPartitionContent struct {
	src       string
	dest      string
	recursive bool
}

func (part RawPartitionOperations) size() uint64 {
	fileStat, err := part.file.Stat()
	if err != nil {
		panic(err)
	}
	return uint64(fileStat.Size())
}

func (part RawPartitionOperations) write(img *disk.Disk, index int) {
	img.WritePartitionContents(index, part.file)
	part.file.Close()
}

func (part FSPartitionOperations) size() uint64 {
	return part.partsize
}

func (part FSPartitionOperations) write(img *disk.Disk, index int) {
	fs, err := img.CreateFilesystem(disk.FilesystemSpec{Partition: index, FSType: part.fsType})
	if err != nil {
		panic(err)
	}

	writeFile := func(src string, dest string) {
		data, err := os.ReadFile(src)
		if err != nil {
			panic(err)
		}
		file, err := fs.OpenFile(dest, os.O_CREATE|os.O_RDWR)
		if err != nil {
			panic(err)
		}
		if _, err := file.Write(data); err != nil {
			panic(err)
		}
		file.Close()
	}

	var writeDir func(srcPath string, destPath string)
	writeDir = func(srcPath string, destPath string) {
		files, err := os.ReadDir(srcPath)
		if err != nil {
			panic(err)
		}

		for _, file := range files {
			fs.Mkdir(destPath)
			writeFunc := writeFile
			if file.IsDir() {
				writeFunc = writeDir
			}
			writeFunc(path.Join(srcPath, file.Name()), path.Join(destPath, file.Name()))
		}
	}

	for _, content := range part.contents {
		stat, err := os.Stat(content.src)
		if err != nil {
			panic(err)
		}

		writeFunc := writeFile
		if stat.IsDir() {
			if !content.recursive {
				panic("expected a file, got directory")
			}
			writeFunc = writeDir
		}
		writeFunc(content.src, content.dest)
	}
}

func BuildImage(config Configuration) {
	imagePath := path.Join(config.dest, config.name)

	if err := os.Remove(imagePath); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	fmt.Printf("Creating image %s\n", config.name)
	var size uint64 = config.sectorSize * config.firstSector
	for _, partition := range config.partitions {
		size += Ceil(partition.ops.size(), config.sectorSize)
	}

	os.MkdirAll(config.dest, 0o755)
	img, err := diskfs.Create(imagePath, int64(size), diskfs.Raw, diskfs.SectorSize(config.sectorSize))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Partitioning...\n")
	currentSector := config.firstSector
	gptPartitions := make([]*gpt.Partition, 0)
	for i, partition := range config.partitions {
		sizeInSectors := Ceil(partition.ops.size(), config.sectorSize) / config.sectorSize
		gptPartition := gpt.Partition{
			Start: currentSector,
			End:   currentSector + sizeInSectors,
			Type:  gpt.Type(partition.typeUUID),
			Name:  partition.name,
		}
		gptPartitions = append(gptPartitions, &gptPartition)
		fmt.Printf("> Partition %d (name: %s, type: %s, start: %d, end: %d)\n", i+1, partition.name, partition.typeUUID, currentSector, currentSector+sizeInSectors)
		currentSector += sizeInSectors
	}

	if err := img.Partition(&gpt.Table{
		Partitions:    gptPartitions,
		ProtectiveMBR: config.pmbr,
	}); err != nil {
		panic(err)
	}

	fmt.Printf("Writing partitions...\n")
	for i, partition := range config.partitions {
		partition.ops.write(img, i+1)
	}

	if config.bootsector != "" {
		fmt.Printf("Writing bootsector...\n")
		bootsector, err := os.ReadFile(config.bootsector)
		if err != nil {
			panic(err)
		}

		bootsectorSize := int64(len(bootsector))
		if bootsectorSize > MaxBootsectorSize {
			panic(fmt.Errorf("bootsector exceeds maximum size of 440 bytes (%d bytes)", bootsectorSize))
		}
		if _, err = img.File.WriteAt(bootsector, 0); err != nil {
			panic(err)
		}
		fmt.Printf("> Bootsector (size: %d)\n", bootsectorSize)
	}

	fmt.Printf("Done\n")
}

func main() {
	configFile := flag.String("config", "mkimg.lua", "Configuration file (lua)")
	dest := flag.String("dest", "./", "Destination directory for the generated image")
	flag.Parse()

	config := Configuration{
		dest:        *dest,
		name:        "default.img",
		pmbr:        false,
		sectorSize:  512,
		firstSector: 2048,
		partitions:  make([]Partition, 0),
		bootsector:  "",
	}

	l := lua.NewState()

	lua.BaseOpen(l)
	lua.MathOpen(l)

	l.NewTable()
	l.PushString("KB")
	l.PushUnsigned(1024)
	l.SetTable(-3)
	l.PushString("MB")
	l.PushUnsigned(1024 * 1024)
	l.SetTable(-3)
	l.PushString("GB")
	l.PushUnsigned(1024 * 1024 * 1024)
	l.SetTable(-3)
	l.SetGlobal("Size")

	l.NewTable()
	l.PushString("Fat32")
	l.PushUnsigned(0)
	l.SetTable(-3)
	l.SetGlobal("FsType")

	l.NewTable()
	l.PushString("Unused")
	l.PushString(string(gpt.Unused))
	l.SetTable(-3)
	l.PushString("ESP")
	l.PushString(string(gpt.EFISystemPartition))
	l.SetTable(-3)
	l.PushString("LegacyMBR")
	l.PushString(string(gpt.MBRPartitionScheme))
	l.SetTable(-3)
	l.SetGlobal("PartType")

	l.Register("SetName", func(state *lua.State) int {
		config.name = lua.CheckString(state, 1)
		return 0
	})

	l.Register("SetSectorSize", func(state *lua.State) int {
		sectorSize := lua.CheckUnsigned(state, 1)
		if sectorSize != 512 && sectorSize != 4096 {
			panic(fmt.Errorf("invalid sector size (use one of 512, 4096): %d", sectorSize))
		}
		config.sectorSize = uint64(sectorSize)
		return 0
	})

	l.Register("SetFirstSector", func(state *lua.State) int {
		config.firstSector = uint64(lua.CheckUnsigned(state, 1))
		return 0
	})

	l.Register("SetBootsector", func(state *lua.State) int {
		config.bootsector = lua.CheckString(state, 1)
		return 0
	})

	l.Register("UseProtectiveMbr", func(state *lua.State) int {
		config.pmbr = true
		return 0
	})

	l.Register("NewRawPartition", func(state *lua.State) int {
		file, err := os.Open(lua.CheckString(state, 3))
		if err != nil {
			if os.IsNotExist(err) {
				panic(fmt.Errorf("partition %d: file %s does not exist", len(config.partitions)+1, file.Name()))
			}
			panic(err)
		}
		config.partitions = append(config.partitions, Partition{
			name:     lua.CheckString(state, 1),
			typeUUID: lua.CheckString(state, 2),
			ops:      RawPartitionOperations{file: file},
		})
		return 0
	})

	l.Register("NewFsPartition", func(state *lua.State) int {
		fspart := FSPartitionOperations{
			partsize: uint64(lua.CheckUnsigned(state, 3)),
			fsType:   filesystem.Type(lua.CheckUnsigned(state, 4)),
			contents: make([]FSPartitionContent, 0),
		}
		partition := Partition{
			name:     lua.CheckString(state, 1),
			typeUUID: lua.CheckString(state, 2),
			ops:      fspart,
		}
		config.partitions = append(config.partitions, partition)

		l.NewTable()
		l.PushGoFunction(func(state *lua.State) int {
			fspart.contents = append(fspart.contents, FSPartitionContent{src: lua.CheckString(state, 2), dest: lua.CheckString(state, 3), recursive: false})
			return 0
		})
		l.SetField(-2, "PutFile")
		l.PushGoFunction(func(state *lua.State) int {
			fspart.contents = append(fspart.contents, FSPartitionContent{src: lua.CheckString(state, 2), dest: lua.CheckString(state, 3), recursive: true})
			return 0
		})
		l.SetField(-2, "PutDir")
		return 1
	})

	if err := lua.DoFile(l, *configFile); err != nil {
		panic(err)
	}

	BuildImage(config)
}
