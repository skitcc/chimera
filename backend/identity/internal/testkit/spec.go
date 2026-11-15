// Package testkit adds the service's builders to the shared chimera/testkit.
package testkit

import base "chimera/testkit"

type (
	Spec   = base.Spec
	Report = base.Report
)

const (
	TechniqueBoundary      = base.TechniqueBoundary
	TechniqueEquivalence   = base.TechniqueEquivalence
	TechniqueState         = base.TechniqueState
	TechniqueDecisionTable = base.TechniqueDecisionTable
	TechniqueErrorGuessing = base.TechniqueErrorGuessing

	LayerDomain  = base.LayerDomain
	LayerUsecase = base.LayerUsecase
	LayerData    = base.LayerData

	KindClassic     = base.KindClassic
	KindLondon      = base.KindLondon
	KindStub        = base.KindStub
	KindPgxmock     = base.KindPgxmock
	KindIntegration = base.KindIntegration

	SeverityBlocker  = base.SeverityBlocker
	SeverityCritical = base.SeverityCritical
	SeverityNormal   = base.SeverityNormal
	SeverityMinor    = base.SeverityMinor
)

var (
	RunSpec = base.RunSpec
	Open    = base.Open
)
