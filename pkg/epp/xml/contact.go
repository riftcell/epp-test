package xml

import "github.com/nbio/xml"

// ContactCheckCommand is the EPP contact:check wire frame for checking handle availability.
type ContactCheckCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:check"`
	// IDs lists the contact handles to check.
	IDs []string `xml:"contact:id"`
}

// ContactCreateCommand is the EPP contact:create wire frame for creating a contact object.
type ContactCreateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:create"`
	// ID is the desired contact handle.
	ID string `xml:"contact:id"`
	// PostalInfo holds the contact's name, organisation, and address.
	PostalInfo ContactPostalInfo `xml:"contact:postalInfo"`
	// Voice is the voice telephone number in +CC.NNNNNNNNNN format.
	Voice string `xml:"contact:voice,omitempty"`
	// Fax is the fax number (optional).
	Fax string `xml:"contact:fax,omitempty"`
	// Email is the contact email address.
	Email string `xml:"contact:email"`
	// AuthInfo holds the contact authorization code.
	AuthInfo ContactAuthInfo `xml:"contact:authInfo"`
}

// ContactPostalInfo holds the name, organisation, and structured postal address.
// Type is either "int" (internationalized ASCII) or "loc" (localized/UTF-8).
type ContactPostalInfo struct {
	// Type is "int" or "loc", placed as an attribute on the postalInfo element.
	Type string `xml:"type,attr"`
	// Name is the contact's full name.
	Name string `xml:"contact:name"`
	// Org is the organisation name (optional).
	Org string `xml:"contact:org,omitempty"`
	// Addr is the structured postal address.
	Addr ContactAddr `xml:"contact:addr"`
}

// ContactAddr is the EPP postal address element within a contact:postalInfo.
type ContactAddr struct {
	// Street lists up to three street address lines.
	Street []string `xml:"contact:street,omitempty"`
	// City is the city or locality.
	City string `xml:"contact:city"`
	// SP is the state or province (optional).
	SP string `xml:"contact:sp,omitempty"`
	// PC is the postal code (optional).
	PC string `xml:"contact:pc,omitempty"`
	// CC is the two-letter ISO 3166 country code.
	CC string `xml:"contact:cc"`
}

// ContactAuthInfo holds the contact authorization code.
type ContactAuthInfo struct {
	// PW is the authorization password string.
	PW string `xml:"contact:pw"`
}

// ContactInfoCommand is the EPP contact:info wire frame for retrieving a contact object.
type ContactInfoCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:info"`
	// ID is the contact handle to retrieve.
	ID string `xml:"contact:id"`
}

// ContactUpdateCommand is the EPP contact:update wire frame for modifying a contact.
type ContactUpdateCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:update"`
	// ID is the contact handle to update.
	ID string `xml:"contact:id"`
	// Chg holds the fields to change on the contact.
	Chg *ContactChange `xml:"contact:chg,omitempty"`
}

// ContactChange holds the mutable fields of a contact that can be updated.
type ContactChange struct {
	// PostalInfo replaces the contact's postal information if non-nil.
	PostalInfo *ContactPostalInfo `xml:"contact:postalInfo,omitempty"`
	// Email replaces the contact email if non-empty.
	Email string `xml:"contact:email,omitempty"`
}

// ContactDeleteCommand is the EPP contact:delete wire frame for removing a contact object.
type ContactDeleteCommand struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:delete"`
	// ID is the contact handle to delete.
	ID string `xml:"contact:id"`
}

// ContactStatus represents an EPP status element with an s attribute.
type ContactStatus struct {
	// S is the status value string (e.g., "ok", "clientDeleteProhibited").
	S string `xml:"s,attr"`
}

// ContactInfoResponse is parsed from the <contact:infData> element in <epp:resData>
// for contact:info responses.
type ContactInfoResponse struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:contact-1.0 contact:infData"`
	// ID is the contact handle.
	ID string `xml:"contact:id"`
	// PostalInfo holds the contact's name, organisation, and address.
	PostalInfo ContactPostalInfo `xml:"contact:postalInfo"`
	// Email is the contact email address.
	Email string `xml:"contact:email"`
	// Statuses lists the EPP status codes on the contact.
	Statuses []ContactStatus `xml:"contact:status"`
}
