# mkimg
mkimg is a tiny tool to simplify the process of creating partitioned disk images. The general idea is to setup a config describing the image you want to create and let mkimg do the work for you.

## Usage
`mkimg [options]`

## Options
`--config=file` overrides the default config file path of _./mkimg.lua_  
`--dest=path` overrides the default outpuit destination  

## Configuration
Configuration for mkimg is done in lua as is it is part of the Elysium family of tools.

### Constants
Size Helpers: `Size.KB`, `Size.MB`, `Size.GB`  
Partition Types: `PartType.Unused`, `PartType.ESP`, `PartType.LegacyMBR`  
FsPartition Filesystem Types: `FsType.Fat32`  

### Functions
Image Configuration:  
`SetName(name: string)`: Set the image name  
`SetSectorSize(size: number)`: Set the disk sector size (only `512` or `4096` are valid)  
`SetFirstSector(sector: number)`: The sector at which to place the first partition  
`SetBootsector(path: string)`: A bootsector binary (max size of 440 bytes)  
`UseProtectiveMbr()`: Wether to write a protective MBR to the image  

Partition Configuration  
`NewRawPartition(name: string, partitionType: string, filePath: string)`: Create a new raw partition (write a file directly to the partition)  
`NewFsPartition(name: string, partitionType: string, size: number, fsType: number): fs`: Create a new filesystem partition  
`fs:PutFile(src: string, dest: string)`: Write a file to a filesystem partition  
`fs:PutDir(src: string, dest: string)`: Write a directory (recursively) to a filesystem partition  

> [!CAUTION]  
> The order in which the partitions are created is the order they will be written to the image.

```lua
-- Image Configuration
SetName("example.lua")
SetSectorSize(512)
SetFirstSector(2048)
SetBootsector("./build/example-sector.bin")
UseProtectiveMbr()

-- Partitions
NewRawPartition("Tartarus", "54524154-5241-5355-424F-4F5450415254", "./build/tartarus.sys")

local espFs = NewFsPartition("ESP", PartType.ESP, Size.GB, FsType.Fat32)
espFs:PutFile("./tartarus.cfg", "/tartarus.cfg")
espFs:PutDir("./build/efidir", "/EFI")

...
```