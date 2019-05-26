package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
)

// ItemStack represents an item instance/stack over network. It has a network ID and a metadata value that
// define its type.
type ItemStack struct {
	// NetworkID is the numerical network ID of the item. This is sometimes a positive ID, and sometimes a
	// negative ID, depending on what item it concerns.
	NetworkID int32
	// MetadataValue is the metadata value of the item. For some items, this is the damage value, whereas for
	// other items it is simply an identifier of a variant of the item.
	MetadataValue int16
	// Count is the count of items that the item stack holds.
	Count int16
	// NBTData is a map that is serialised to its NBT representation when sent in a packet.
	NBTData map[string]interface{}
	// CanBePlacedOn is a list of block identifiers like 'minecraft:stone' which the item, if it is an item
	// that can be placed, can be placed on top of.
	CanBePlacedOn []string
	// CanBreak is a list of block identifiers like 'minecraft:dirt' that the item is able to break.
	CanBreak []string
}

// Item reads an item stack from buffer src and stores it into item stack x.
func Item(src *bytes.Buffer, x *ItemStack) error {
	if err := Varint32(src, &x.NetworkID); err != nil {
		return err
	}
	if x.NetworkID == 0 {
		// The item was air, so there is no more data we should read for the item instance. After all, air
		// items aren't really anything.
		return nil
	}
	var auxValue int32
	if err := Varint32(src, &auxValue); err != nil {
		return err
	}
	x.MetadataValue = int16(auxValue >> 8)
	x.Count = int16(auxValue & 0xff)

	var legacyNBTLength int16
	if err := binary.Read(src, binary.LittleEndian, &legacyNBTLength); err != nil {
		return err
	}
	// This legacy NBT length was, before 1.9, the length of the NBT to follow, but this is no longer the
	// case. It is now always -1.
	if legacyNBTLength != -1 {
		return fmt.Errorf("expected legacy NBT length to be -1, got %v", legacyNBTLength)
	}

	var nbtCount byte
	if err := binary.Read(src, binary.LittleEndian, &nbtCount); err != nil {
		return err
	}
	if nbtCount != 1 {
		// The NBT count seems to be always 1, so we return an error if it is not, just so we know there can
		// be more than one.
		return fmt.Errorf("expected NBT count to be 1, got %v", nbtCount)
	}
	decoder := nbt.NewDecoder(src)
	for i := byte(0); i < nbtCount; i++ {
		if err := decoder.Decode(&x.NBTData); err != nil {
			return fmt.Errorf("error decoding item NBT: %v", err)
		}
	}
	var length uint32
	if err := Varuint32(src, &length); err != nil {
		return err
	}
	x.CanBePlacedOn = make([]string, length)
	for i := uint32(0); i < length; i++ {
		if err := String(src, &x.CanBePlacedOn[i]); err != nil {
			return err
		}
	}
	if err := Varuint32(src, &length); err != nil {
		return err
	}
	x.CanBreak = make([]string, length)
	for i := uint32(0); i < length; i++ {
		if err := String(src, &x.CanBreak[i]); err != nil {
			return err
		}
	}
	const shieldID = 513
	if x.NetworkID == shieldID {
		var blockingTick int64
		return Varint64(src, &blockingTick)
	}
	return nil
}

// WriteItem writes an item stack x to buffer dst.
func WriteItem(dst *bytes.Buffer, x ItemStack) error {
	if err := WriteVarint32(dst, x.NetworkID); err != nil {
		return err
	}
	if x.NetworkID == 0 {
		// The item was air, so there's no more data to follow. Return immediately.
		return nil
	}
	if err := WriteVarint32(dst, int32(x.MetadataValue<<8)|int32(x.Count)); err != nil {
		return err
	}
	// Write a fixed -1, which used to be the NBT length.
	if err := binary.Write(dst, binary.LittleEndian, int16(-1)); err != nil {
		return err
	}
	// NBT Count, which is always one in our case.
	if err := binary.Write(dst, binary.LittleEndian, byte(1)); err != nil {
		return err
	}
	if _, err := nbt.Marshal(x.NBTData); err != nil {
		return fmt.Errorf("error writing NBT: %v", err)
	}
	if err := WriteVaruint32(dst, uint32(len(x.CanBePlacedOn))); err != nil {
		return err
	}
	for _, block := range x.CanBePlacedOn {
		if err := WriteString(dst, block); err != nil {
			return err
		}
	}
	if err := WriteVaruint32(dst, uint32(len(x.CanBreak))); err != nil {
		return err
	}
	for _, block := range x.CanBreak {
		if err := WriteString(dst, block); err != nil {
			return err
		}
	}
	const shieldID = 513
	if x.NetworkID == shieldID {
		var blockingTick int64
		return WriteVarint64(dst, blockingTick)
	}

	return nil
}