// Package nicat provides the NiC.AT (.at) EPP registrar adapter (REG-03).
//
// NiCATAdapter is the NiC.AT (.at) registrar adapter. It wraps GenericEPPAdapter
// and sets the contact-create hook that injects the at-ext-verification-1.0
// contact verification extension.
package nicat
