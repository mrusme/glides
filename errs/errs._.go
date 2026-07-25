package errs

import "errors"

var (
	ErrNotImplemented error = errors.New(
		"err_not_implemented",
	)
	ErrConfigTypeUnsupported error = errors.New(
		"err_config_type_unsupported",
	)
	ErrIfaceInvalid error = errors.New(
		"err_iface_invalid",
	)
	ErrNoRows error = errors.New(
		"no_rows",
	)
	ErrUniqueViolationOn error = errors.New(
		"unique_violation_on",
	)
	ErrHashInvalid error = errors.New(
		"err_hash_invalid",
	)
	ErrHashVariantIncompatible error = errors.New(
		"err_hash_variant_incompatible",
	)
	ErrHashVersionIncompatible error = errors.New(
		"err_hash_version_incompatible",
	)
	ErrStorageIDNotFound error = errors.New(
		"err_storage_id_not_found",
	)
	ErrStorageTypeInvalid error = errors.New(
		"err_storage_type_invalid",
	)
	ErrFilePathInvalid error = errors.New(
		"err_file_path_invalid",
	)
	ErrTargetIDNotFound error = errors.New(
		"err_target_id_not_found",
	)
	ErrNoSuchTargetType error = errors.New(
		"err_no_such_target_type",
	)
	ErrJobTypeInvalid error = errors.New(
		"err_job_type_invalid",
	)
	ErrJobSubTypeInvalid error = errors.New(
		"err_job_sub_type_invalid",
	)
	ErrJobPayloadInvalid error = errors.New(
		"err_job_payload_invalid",
	)
	ErrDispatchResolverMissing error = errors.New(
		"err_dispatch_resolver_missing",
	)
	ErrCronFunctionIDExists error = errors.New(
		"err_cron_function_id_exists",
	)
	ErrCronFunctionIDNotFound error = errors.New(
		"err_cron_function_id_not_found",
	)
	ErrCronFunctionInvalid error = errors.New(
		"err_cron_function_invalid",
	)
)
