package dashboard

import (
	"agent-desk/internal/pkg/errorsx"
	"errors"
)

func localizedExperienceError(err error) error {
	var localized *errorsx.I18nError
	if errors.As(err, &localized) {
		return localized
	}
	return errorsx.InvalidParamI18n("error.salesExperience.failed")
}
