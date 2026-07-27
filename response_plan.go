package main

import "encoding/binary"

type responseRecord struct {
	Name               string
	Type               uint16
	Record             Record
	UseQuestionPointer bool
}

type responsePlan struct {
	RCode         uint16
	Authoritative bool
	Answers       []responseRecord
	Authorities   []responseRecord
}

func planResponse(question Question, zone Zone) responsePlan {
	name := canonicalName(question.Name)
	if question.QClass != ClassIN {
		return responsePlan{}
	}

	if !zone.contains(name) {
		return responsePlan{RCode: rCodeRefused}
	}

	plan := responsePlan{Authoritative: true}
	if !zone.nameExists(name) {
		plan.RCode = rCodeNXDomain
		if soa, ok := negativeSOARecord(zone); ok {
			plan.Authorities = append(plan.Authorities, soa)
		}
		return plan
	}

	for _, record := range zone.Records[name][question.QType] {
		plan.Answers = append(plan.Answers, responseRecord{
			Name:               name,
			Type:               question.QType,
			Record:             record,
			UseQuestionPointer: true,
		})
	}

	if len(plan.Answers) == 0 {
		if soa, ok := negativeSOARecord(zone); ok {
			plan.Authorities = append(plan.Authorities, soa)
		}
	}

	return plan
}

func negativeSOARecord(zone Zone) (responseRecord, bool) {
	origin := canonicalName(zone.Origin)
	records := zone.Records[origin][TypeSOA]
	if len(records) == 0 {
		return responseRecord{}, false
	}

	record := records[0]
	if len(record.RData) >= 4 {
		minimum := binary.BigEndian.Uint32(record.RData[len(record.RData)-4:])
		if minimum < record.TTL {
			record.TTL = minimum
		}
	}

	return responseRecord{
		Name:   origin,
		Type:   TypeSOA,
		Record: record,
	}, true
}
