// gonest/validator/rules/date.go
package rules

import (
	"time"

	"github.com/gonest-dev/gonest/validator"
)

// DateAfter validates that date is after the specified date
func DateAfter(after time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.After(after) {
			return validator.
				NewFieldError("", "date_after", "Date must be after the specified date").
				WithParam("after", after).
				WithParam("actual", value)
		}
		return nil
	}
}

// DateBefore validates that date is before the specified date
func DateBefore(before time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.Before(before) {
			return validator.
				NewFieldError("", "date_before", "Date must be before the specified date").
				WithParam("before", before).
				WithParam("actual", value)
		}
		return nil
	}
}

// DateBetween validates that date is between two dates
func DateBetween(start, end time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if value.Before(start) || value.After(end) {
			return validator.
				NewFieldError("", "date_between", "Date must be between the specified dates").
				WithParam("start", start).
				WithParam("end", end).
				WithParam("actual", value)
		}
		return nil
	}
}

// DatePast validates that date is in the past
func DatePast() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.Before(time.Now()) {
			return validator.
				NewFieldError("", "date_past", "Date must be in the past").
				WithParam("actual", value).
				WithParam("now", time.Now())
		}
		return nil
	}
}

// DateFuture validates that date is in the future
func DateFuture() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.After(time.Now()) {
			return validator.
				NewFieldError("", "date_future", "Date must be in the future").
				WithParam("actual", value).
				WithParam("now", time.Now())
		}
		return nil
	}
}

// DateToday validates that date is today
func DateToday() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		tomorrow := today.AddDate(0, 0, 1)

		if value.Before(today) || value.After(tomorrow) {
			return validator.
				NewFieldError("", "date_today", "Date must be today").
				WithParam("actual", value).
				WithParam("today", today)
		}
		return nil
	}
}

// DateMinAge validates minimum age from birthdate
func DateMinAge(minAge int) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		age := now.Year() - value.Year()

		// Adjust if birthday hasn't occurred this year
		if now.YearDay() < value.YearDay() {
			age--
		}

		if age < minAge {
			return validator.
				NewFieldError("", "date_min_age", "Age is below minimum").
				WithParam("min_age", minAge).
				WithParam("actual_age", age).
				WithParam("birthdate", value)
		}
		return nil
	}
}

// DateMaxAge validates maximum age from birthdate
func DateMaxAge(maxAge int) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		age := now.Year() - value.Year()

		if now.YearDay() < value.YearDay() {
			age--
		}

		if age > maxAge {
			return validator.
				NewFieldError("", "date_max_age", "Age is above maximum").
				WithParam("max_age", maxAge).
				WithParam("actual_age", age).
				WithParam("birthdate", value)
		}
		return nil
	}
}

// DateWeekday validates that date is a specific weekday
func DateWeekday(weekday time.Weekday) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if value.Weekday() != weekday {
			return validator.
				NewFieldError("", "date_weekday", "Date must be on the specified weekday").
				WithParam("expected", weekday.String()).
				WithParam("actual", value.Weekday().String())
		}
		return nil
	}
}

// DateWeekend validates that date is on weekend (Saturday or Sunday)
func DateWeekend() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		weekday := value.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			return validator.
				NewFieldError("", "date_weekend", "Date must be on weekend").
				WithParam("actual", weekday.String())
		}
		return nil
	}
}

// DateWeekday validates that date is on a weekday (Monday-Friday)
func DateIsWeekday() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		weekday := value.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return validator.
				NewFieldError("", "date_weekday", "Date must be on a weekday").
				WithParam("actual", weekday.String())
		}
		return nil
	}
}
