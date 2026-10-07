package diskinventory

import "strings"

// HardwareIdentityConflict reports whether two observations of one disk slot
// carry hardware identities that name different disks: nothing agrees across
// serial and WWN (HardwareIdentityMatch), and at least one field reported on
// both sides disagrees. A field missing or unusable on either side is no
// evidence, so an observation without identity never conflicts.
//
// Reporters spell one identity differently, and spelling is not disagreement:
//   - serials are compared on letters and digits only, because udev rewrites
//     whitespace and unusual characters in a serial as underscores while
//     smartctl keeps them, and a T10 vendor designator for an ATA drive
//     ("ATA", the model, then the drive serial) is no evidence against the
//     serial it ends with, because udev reports one when a SCSI-ATA
//     translation layer hides IDENTIFY from it;
//   - a WWN that begins with the other is no evidence, because udev's ID_WWN
//     carries only the first 64 bits of a 128-bit NAA 6 identifier that sysfs
//     and smartctl report in full.
//
// A caller blanks a field its producer may not report as disk identity, such
// as a SAS disk's serial, where Proxmox may report a SAS address, and checks
// HardwareIdentityMatch with the field kept if it may still show agreement.
func HardwareIdentityConflict(leftSerial, leftWWN, rightSerial, rightWWN string) bool {
	if HardwareIdentityMatch(leftSerial, leftWWN, rightSerial, rightWWN) {
		return false
	}
	return serialsDisagree(leftSerial, rightSerial) || wwnsDisagree(leftWWN, rightWWN)
}

func serialsDisagree(left, right string) bool {
	left, right = compactSerial(normalizeHardwareID(left)), compactSerial(normalizeHardwareID(right))
	return left != "" && right != "" && left != right &&
		!isATADesignatorOf(left, right) && !isATADesignatorOf(right, left)
}

// isATADesignatorOf reports whether designator, compacted, is the T10 vendor
// designator of the ATA drive whose serial is given: the "ata" vendor field,
// a model, then that serial.
func isATADesignatorOf(designator, serial string) bool {
	const vendor = "ata"
	return len(designator) > len(vendor)+len(serial) &&
		strings.HasPrefix(designator, vendor) && strings.HasSuffix(designator, serial)
}

func compactSerial(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func wwnsDisagree(left, right string) bool {
	left, right = normalizeHardwareWWN(left), normalizeHardwareWWN(right)
	return left != "" && right != "" &&
		!strings.HasPrefix(left, right) && !strings.HasPrefix(right, left)
}
