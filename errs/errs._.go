package errs

import "errors"

var (
	ErrNotImplemented error = errors.New(
		"err_not_implemented",
	)
	ErrConfigTypeUnsupported error = errors.New(
		"err_config_type_unsupported",
	)
	ErrConfigModeInvalid error = errors.New(
		"err_config_mode_invalid",
	)
	ErrProcessState error = errors.New(
		"err_process_state",
	)
	ErrIfaceInvalid error = errors.New(
		"err_iface_invalid",
	)
	ErrServiceNotFound error = errors.New(
		"err_service_not_found",
	)
	ErrEmbedNotFound error = errors.New(
		"err_embed_not_found",
	)
	ErrNoRows error = errors.New(
		"no_rows",
	)
	ErrDatabaseNotStarted error = errors.New(
		"err_database_not_started",
	)
	ErrUniqueViolationOn error = errors.New(
		"unique_violation_on",
	)
	ErrUnavailable error = errors.New(
		"err_unavailable",
	)
	ErrRowMismatch error = errors.New(
		"err_row_mismatch",
	)
	ErrDatabaseReadOnly error = errors.New(
		"err_database_read_only",
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
	ErrStorageIDDuplicate error = errors.New(
		"err_storage_id_duplicate",
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
	ErrTargetIDDuplicate error = errors.New(
		"err_target_id_duplicate",
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
	ErrDispatchNotStarted error = errors.New(
		"err_dispatch_not_started",
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
	ErrXMPPClosed error = errors.New(
		"err_xmpp_closed",
	)
	ErrXMPPPingTimeout error = errors.New(
		"err_xmpp_ping_timeout",
	)
)
