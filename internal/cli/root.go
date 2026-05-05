package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mirelahmd/OpenVFX/internal/commands"
	"github.com/mirelahmd/OpenVFX/internal/config"
)

const usage = `byom-video is a local-first media/video workflow control plane.

Usage:
  byom-video doctor
  byom-video version
  byom-video init [--force]
  byom-video config show [--json]
  byom-video models [--json]
  byom-video models validate [--json]
  byom-video models doctor [--json]
  byom-video run <input-file> [--with-transcript-stub | --with-transcript] [--with-captions] [--with-chunks] [--with-highlights] [--with-roughcut] [--with-ffmpeg-script] [--ffmpeg-mode <stream-copy|reencode>] [--with-report]
  byom-video pipeline <input-file> --preset <shorts|metadata>
  byom-video batch <input-dir> [--preset <shorts|metadata>] [--recursive] [--limit <n>] [--fail-fast] [--dry-run] [--validate] [--export | --export-and-validate]
  byom-video batches
  byom-video inspect-batch <batch_id> [--json]
  byom-video watch <input-dir> [--preset <shorts|metadata>] [--interval-seconds <n>] [--recursive] [--once] [--limit <n>] [--fail-fast] [--validate] [--export | --export-and-validate] [--ignore-registry]
  byom-video watch-status [--json]
  byom-video retry-batch <batch_id> [--limit <n>] [--fail-fast] [--dry-run] [--validate] [--export | --export-and-validate]
  byom-video retry-watch [--preset <shorts|metadata>] [--limit <n>] [--fail-fast] [--dry-run] [--validate] [--export | --export-and-validate]
  byom-video rerun <run_id> [--preset <shorts|metadata>] [--dry-run] [--validate] [--export | --export-and-validate]
  byom-video cleanup [--failed] [--stale-running] [--missing-manifest] [--older-than-hours <n>] [--limit <n>] [--json] [--delete] [--yes]
  byom-video plan <path> --goal <text> [--mode <file|batch|watch>] [--execute] [--dry-run] [--preset <shorts|metadata>] [--max-clips <n>] [--recursive] [--once] [--limit <n>] [--with-export] [--with-validate] [--with-report]
  byom-video plans
  byom-video inspect-plan <plan_id> [--json]
  byom-video plan-artifacts <plan_id> [--json]
  byom-video review-plan <plan_id> [--json] [--write-artifact]
  byom-video approve-plan <plan_id>
  byom-video execute-plan <plan_id> [--yes] [--dry-run]
  byom-video diff-plan <plan_id_a> <plan_id_b> [--json] [--write-artifact]
  byom-video revise-plan <plan_id> --request <text> [--dry-run] [--json] [--show-diff]
  byom-video snapshots <plan_id>
  byom-video inspect-snapshot <plan_id> <snapshot_id> [--json]
  byom-video diff-snapshot <plan_id> <snapshot_id> [--json] [--write-artifact]
  byom-video runs [--limit <n> | --all]
  byom-video inspect <run_id> [--json]
  byom-video artifacts <run_id> [--type <name>]
  byom-video validate <run_id> [--json]
  byom-video clip-cards <run_id> [--overwrite] [--json]
  byom-video review-clips <run_id> [--json] [--write-artifact]
  byom-video enhance-roughcut <run_id> [--overwrite] [--json]
  byom-video selected-clips <run_id> [--overwrite] [--json]
  byom-video export-manifest <run_id> [--overwrite] [--json]
  byom-video ffmpeg-script <run_id> [--mode <stream-copy|reencode>] [--overwrite] [--json]
  byom-video concat-plan <run_id> [--overwrite] [--json]
  byom-video mask-template <run_id>
  byom-video inspect-mask <run_id> [--json]
  byom-video mask-validate <run_id> [--json]
  byom-video mask-plan <run_id> [--intent <text>] [--tone <text>] [--max-caption-words <n>] [--top-k <n>] [--overwrite]
  byom-video review-mask <run_id> [--json] [--write-artifact]
  byom-video expansion-plan <run_id> [--caption-variants <n>] [--label-max-words <n>] [--description-max-words <n>] [--overwrite]
  byom-video verification-plan <run_id> [--overwrite]
  byom-video routes-plan <run_id> [--json] [--write-artifact] [--strict]
  byom-video revise-mask <run_id> --request <text> [--dry-run] [--json] [--show-diff]
  byom-video mask-snapshots <run_id> [--json]
  byom-video inspect-mask-snapshot <run_id> <snapshot_id> [--json]
  byom-video diff-mask <run_id> <snapshot_id> [--json] [--write-artifact]
  byom-video mask-decisions <run_id> [--json]
  byom-video mask-decision <run_id> <decision_id> --set <keep|reject|candidate_keep> [--reason <text>] [--dry-run] [--json]
  byom-video mask-remove-decision <run_id> <decision_id> [--dry-run] [--json]
  byom-video mask-reorder <run_id> --order <decision_id,...> [--dry-run] [--json]
  byom-video route-preview <run_id> [--json] [--write-artifact]
  byom-video expand-dry-run <run_id> [--json] [--strict] [--task-type <caption_variants|timeline_labels|short_descriptions>]
  byom-video expand <run_id> [--overwrite] [--json] [--task-type <caption_variants|timeline_labels|short_descriptions>] [--strict] [--dry-run] [--max-tasks <n>] [--fail-fast]
  byom-video review-model-requests <run_id> [--json] [--write-artifact]
  byom-video expand-local-stub <run_id> [--overwrite] [--json] [--task-type <caption_variants|timeline_labels|short_descriptions>]
  byom-video expand-stub <run_id> [--overwrite] [--json] [--task-type <caption_variants|timeline_labels|short_descriptions>]
  byom-video expansion-validate <run_id> [--json]
  byom-video review-expansions <run_id> [--json] [--write-artifact]
  byom-video verify-expansions <run_id> [--json] [--tolerance-seconds <n>]
  byom-video review-verification <run_id> [--json] [--write-artifact]
  byom-video export <run_id>
  byom-video open-report <run_id> [--open]
`

func Execute(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "doctor":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: doctor does not accept arguments")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Doctor(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: version does not accept arguments")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VersionCommand(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "init":
		force, err := parseInitArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Init(stdout, force); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "config":
		opts, err := parseConfigArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ConfigShow(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "models":
		opts, err := parseModelsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if opts.Validate {
			if err := commands.ModelsValidate(stdout, commands.ModelsValidateOptions{JSON: opts.JSON}); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		if opts.Doctor {
			if err := commands.ModelsDoctor(stdout, commands.ModelsDoctorOptions{JSON: opts.JSON}); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		if err := commands.Models(stdout, commands.ModelsOptions{JSON: opts.JSON}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "run":
		base, err := configuredRunOptions(true)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		inputFile, opts, err := parseRunArgsWithBase(args[1:], base)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Run(inputFile, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "pipeline":
		inputFile, opts, err := parsePipelineArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Run(inputFile, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "batch":
		inputDir, opts, err := parseBatchArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Batch(inputDir, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "batches":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: batches does not accept arguments")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Batches(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-batch":
		batchID, opts, err := parseInspectBatchArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectBatch(batchID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "watch":
		inputDir, opts, err := parseWatchArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Watch(inputDir, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "watch-status":
		opts, err := parseWatchStatusArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.WatchStatus(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "retry-batch":
		batchID, opts, err := parseRetryBatchArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RetryBatch(batchID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "retry-watch":
		opts, err := parseRetryWatchArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RetryWatch(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "rerun":
		runID, opts, err := parseRerunArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Rerun(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "cleanup":
		opts, err := parseCleanupArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Cleanup(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "plan":
		inputFile, opts, err := parsePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Plan(inputFile, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "plans":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: plans does not accept arguments")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Plans(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-plan":
		planID, opts, err := parseInspectPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "plan-artifacts":
		planID, opts, err := parsePlanArtifactsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.PlanArtifacts(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-plan":
		planID, opts, err := parseReviewPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "approve-plan":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "error: approve-plan requires exactly one plan id")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ApprovePlan(args[1], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "execute-plan":
		planID, opts, err := parseExecutePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExecuteSavedPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "diff-plan":
		a, b, opts, err := parseDiffPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.DiffPlan(a, b, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "revise-plan":
		planID, opts, err := parseRevisePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RevisePlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "snapshots":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "error: snapshots requires exactly one plan id")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Snapshots(args[1], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-snapshot":
		planID, snapshotID, opts, err := parseInspectSnapshotArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectSnapshot(planID, snapshotID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "diff-snapshot":
		planID, snapshotID, opts, err := parseDiffSnapshotArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.DiffSnapshot(planID, snapshotID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "runs":
		opts, err := parseRunsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Runs(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect":
		runID, opts, err := parseInspectArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Inspect(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "artifacts":
		runID, opts, err := parseArtifactsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Artifacts(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "validate":
		runID, opts, err := parseValidateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Validate(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "clip-cards":
		runID, opts, err := parseClipCardsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ClipCardsCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-clips":
		runID, opts, err := parseReviewClipsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewClips(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "enhance-roughcut":
		runID, opts, err := parseEnhanceRoughcutArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.EnhanceRoughcut(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "selected-clips":
		runID, opts, err := parseSelectedClipsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.SelectedClipsCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "export-manifest":
		runID, opts, err := parseExportManifestArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExportManifestCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "ffmpeg-script":
		runID, opts, err := parseFFmpegScriptCommandArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.FFmpegScriptCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "concat-plan":
		runID, opts, err := parseConcatPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ConcatPlanCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-template":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "error: mask-template requires exactly one run id")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskTemplate(args[1], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-plan":
		runID, opts, err := parseMaskPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskPlan(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-mask":
		runID, opts, err := parseInspectMaskArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectMask(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-mask":
		runID, opts, err := parseReviewMaskArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewMask(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expansion-plan":
		runID, opts, err := parseExpansionPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExpansionPlanCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "verification-plan":
		runID, opts, err := parseVerificationPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VerificationPlanCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-validate":
		runID, opts, err := parseMaskValidateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskValidate(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "routes-plan":
		runID, opts, err := parseRoutesPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RoutesPlanCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "revise-mask":
		runID, opts, err := parseReviseMaskArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviseMask(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-snapshots":
		runID, opts, err := parseMaskSnapshotsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskSnapshots(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-mask-snapshot":
		runID, snapshotID, opts, err := parseInspectMaskSnapshotArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectMaskSnapshot(runID, snapshotID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "diff-mask":
		runID, snapshotID, opts, err := parseDiffMaskArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.DiffMask(runID, snapshotID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-decisions":
		runID, opts, err := parseMaskDecisionsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskDecisionsList(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-decision":
		runID, decisionID, opts, err := parseMaskDecisionArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskDecisionCommand(runID, decisionID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-remove-decision":
		runID, decisionID, opts, err := parseMaskRemoveDecisionArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskRemoveDecisionCommand(runID, decisionID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "mask-reorder":
		runID, opts, err := parseMaskReorderArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MaskReorderCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "route-preview":
		runID, opts, err := parseRoutePreviewArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RoutePreviewCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expand-dry-run":
		runID, opts, err := parseExpandDryRunArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExpandDryRun(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expand":
		runID, opts, err := parseExpandArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Expand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-model-requests":
		runID, opts, err := parseReviewModelRequestsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewModelRequests(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expand-local-stub":
		runID, opts, err := parseExpandLocalStubArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExpandLocalStub(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expand-stub":
		runID, opts, err := parseExpandStubArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExpandStub(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "expansion-validate":
		runID, opts, err := parseExpansionValidateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExpansionValidate(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-expansions":
		runID, opts, err := parseReviewExpansionsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewExpansions(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "verify-expansions":
		runID, opts, err := parseVerifyExpansionsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VerifyExpansions(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-verification":
		runID, opts, err := parseReviewVerificationArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewVerification(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "export":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "error: export requires exactly one run id")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Export(args[1], stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "open-report":
		runID, open, err := parseOpenReportArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.OpenReport(runID, stdout, open); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func parseRunArgs(args []string) (string, commands.RunOptions, error) {
	return parseRunArgsWithBase(args, defaultRunOptions())
}

func defaultRunOptions() commands.RunOptions {
	return commands.RunOptions{
		TranscriptModelSize:  "tiny",
		ChunkTargetSeconds:   30,
		ChunkMaxGapSeconds:   2.0,
		HighlightTopK:        10,
		HighlightMinDuration: 3,
		HighlightMaxDuration: 90,
		RoughcutMaxClips:     5,
		FFmpegOutputFormat:   "mp4",
		FFmpegMode:           "stream-copy",
	}
}

func parseRunArgsWithBase(args []string, opts commands.RunOptions) (string, commands.RunOptions, error) {
	var inputFile string

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--with-transcript-stub":
			opts.WithTranscriptStub = true
		case "--with-transcript":
			opts.WithTranscript = true
		case "--with-captions":
			opts.WithCaptions = true
		case "--with-chunks":
			opts.WithChunks = true
		case "--with-highlights":
			opts.WithHighlights = true
		case "--with-roughcut":
			opts.WithRoughcut = true
		case "--with-ffmpeg-script":
			opts.WithFFmpegScript = true
		case "--with-report":
			opts.WithReport = true
		case "--transcript-model-size":
			if index+1 >= len(args) {
				return "", opts, errors.New("--transcript-model-size requires a value")
			}
			index++
			opts.TranscriptModelSize = args[index]
			opts.TranscriptModelSizeSet = true
		case "--chunk-target-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--chunk-target-seconds requires a value")
			}
			index++
			value, err := parseFloatFlag("--chunk-target-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.ChunkTargetSeconds = value
			opts.ChunkTargetSecondsSet = true
		case "--chunk-max-gap-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--chunk-max-gap-seconds requires a value")
			}
			index++
			value, err := parseFloatFlag("--chunk-max-gap-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.ChunkMaxGapSeconds = value
			opts.ChunkMaxGapSecondsSet = true
		case "--highlight-top-k":
			if index+1 >= len(args) {
				return "", opts, errors.New("--highlight-top-k requires a value")
			}
			index++
			value, err := parseIntFlag("--highlight-top-k", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.HighlightTopK = value
			opts.HighlightTopKSet = true
		case "--highlight-min-duration-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--highlight-min-duration-seconds requires a value")
			}
			index++
			value, err := parseFloatFlag("--highlight-min-duration-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.HighlightMinDuration = value
			opts.HighlightMinDurationSet = true
		case "--highlight-max-duration-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--highlight-max-duration-seconds requires a value")
			}
			index++
			value, err := parseFloatFlag("--highlight-max-duration-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.HighlightMaxDuration = value
			opts.HighlightMaxDurationSet = true
		case "--roughcut-max-clips":
			if index+1 >= len(args) {
				return "", opts, errors.New("--roughcut-max-clips requires a value")
			}
			index++
			value, err := parseIntFlag("--roughcut-max-clips", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.RoughcutMaxClips = value
			opts.RoughcutMaxClipsSet = true
		case "--ffmpeg-output-format":
			if index+1 >= len(args) {
				return "", opts, errors.New("--ffmpeg-output-format requires a value")
			}
			index++
			opts.FFmpegOutputFormat = args[index]
			opts.FFmpegOutputFormatSet = true
		case "--ffmpeg-mode":
			if index+1 >= len(args) {
				return "", opts, errors.New("--ffmpeg-mode requires a value")
			}
			index++
			opts.FFmpegMode = args[index]
			opts.FFmpegModeSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--transcript-model-size="); ok {
				opts.TranscriptModelSize = value
				opts.TranscriptModelSizeSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--chunk-target-seconds="); ok {
				parsed, err := parseFloatFlag("--chunk-target-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.ChunkTargetSeconds = parsed
				opts.ChunkTargetSecondsSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--chunk-max-gap-seconds="); ok {
				parsed, err := parseFloatFlag("--chunk-max-gap-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.ChunkMaxGapSeconds = parsed
				opts.ChunkMaxGapSecondsSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--highlight-top-k="); ok {
				parsed, err := parseIntFlag("--highlight-top-k", value)
				if err != nil {
					return "", opts, err
				}
				opts.HighlightTopK = parsed
				opts.HighlightTopKSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--highlight-min-duration-seconds="); ok {
				parsed, err := parseFloatFlag("--highlight-min-duration-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.HighlightMinDuration = parsed
				opts.HighlightMinDurationSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--highlight-max-duration-seconds="); ok {
				parsed, err := parseFloatFlag("--highlight-max-duration-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.HighlightMaxDuration = parsed
				opts.HighlightMaxDurationSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--roughcut-max-clips="); ok {
				parsed, err := parseIntFlag("--roughcut-max-clips", value)
				if err != nil {
					return "", opts, err
				}
				opts.RoughcutMaxClips = parsed
				opts.RoughcutMaxClipsSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--ffmpeg-output-format="); ok {
				opts.FFmpegOutputFormat = value
				opts.FFmpegOutputFormatSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--ffmpeg-mode="); ok {
				opts.FFmpegMode = value
				opts.FFmpegModeSet = true
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown run flag %q", arg)
			}
			if inputFile != "" {
				return "", opts, errors.New("run requires exactly one input file")
			}
			inputFile = arg
		}
	}

	if inputFile == "" {
		return "", opts, errors.New("run requires exactly one input file")
	}
	if opts.WithTranscript && opts.WithTranscriptStub {
		return "", opts, errors.New("--with-transcript and --with-transcript-stub are mutually exclusive")
	}
	if opts.WithCaptions && !opts.WithTranscript && !opts.WithTranscriptStub {
		return "", opts, errors.New("--with-captions requires --with-transcript or --with-transcript-stub")
	}
	if !opts.WithTranscript && opts.TranscriptModelSizeSet {
		return "", opts, errors.New("--transcript-model-size requires --with-transcript")
	}
	if err := commands.ValidateTranscriptModelSize(opts.TranscriptModelSize); err != nil {
		return "", opts, err
	}
	if opts.WithChunks && !opts.WithTranscript && !opts.WithTranscriptStub {
		return "", opts, errors.New("--with-chunks requires --with-transcript or --with-transcript-stub")
	}
	if opts.WithRoughcut && opts.WithChunks {
		opts.WithHighlights = true
	}
	if opts.WithHighlights && !opts.WithChunks {
		return "", opts, errors.New("--with-highlights requires --with-chunks")
	}
	if opts.WithRoughcut && !opts.WithChunks {
		return "", opts, errors.New("--with-roughcut requires --with-chunks")
	}
	if opts.WithFFmpegScript && !opts.WithRoughcut {
		return "", opts, errors.New("--with-ffmpeg-script requires --with-roughcut")
	}
	if !opts.WithChunks && opts.ChunkTargetSecondsSet {
		return "", opts, errors.New("--chunk-target-seconds requires --with-chunks")
	}
	if !opts.WithChunks && opts.ChunkMaxGapSecondsSet {
		return "", opts, errors.New("--chunk-max-gap-seconds requires --with-chunks")
	}
	if opts.ChunkTargetSeconds <= 0 {
		return "", opts, errors.New("--chunk-target-seconds must be positive")
	}
	if opts.ChunkMaxGapSeconds < 0 {
		return "", opts, errors.New("--chunk-max-gap-seconds must be non-negative")
	}
	if !opts.WithHighlights && !opts.WithRoughcut && (opts.HighlightTopKSet || opts.HighlightMinDurationSet || opts.HighlightMaxDurationSet) {
		return "", opts, errors.New("highlight flags require --with-highlights or --with-roughcut")
	}
	if opts.HighlightTopK <= 0 {
		return "", opts, errors.New("--highlight-top-k must be positive")
	}
	if opts.HighlightMinDuration < 0 {
		return "", opts, errors.New("--highlight-min-duration-seconds must be non-negative")
	}
	if opts.HighlightMaxDuration <= opts.HighlightMinDuration {
		return "", opts, errors.New("--highlight-max-duration-seconds must be greater than --highlight-min-duration-seconds")
	}
	if !opts.WithRoughcut && opts.RoughcutMaxClipsSet {
		return "", opts, errors.New("--roughcut-max-clips requires --with-roughcut")
	}
	if opts.RoughcutMaxClips <= 0 {
		return "", opts, errors.New("--roughcut-max-clips must be positive")
	}
	if !opts.WithFFmpegScript && opts.FFmpegOutputFormatSet {
		return "", opts, errors.New("--ffmpeg-output-format requires --with-ffmpeg-script")
	}
	if !opts.WithFFmpegScript && opts.FFmpegModeSet {
		return "", opts, errors.New("--ffmpeg-mode requires --with-ffmpeg-script")
	}
	if opts.FFmpegOutputFormat != "mp4" {
		return "", opts, fmt.Errorf("unsupported ffmpeg output format %q; supported values: mp4", opts.FFmpegOutputFormat)
	}
	if opts.FFmpegMode != "" && opts.FFmpegMode != "stream-copy" && opts.FFmpegMode != "reencode" {
		return "", opts, fmt.Errorf("unsupported ffmpeg mode %q; supported values: stream-copy, reencode", opts.FFmpegMode)
	}
	return inputFile, opts, nil
}

func parseInitArgs(args []string) (bool, error) {
	force := false
	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		default:
			return false, fmt.Errorf("unknown init flag %q", arg)
		}
	}
	return force, nil
}

func parseConfigArgs(args []string) (commands.ConfigShowOptions, error) {
	opts := commands.ConfigShowOptions{}
	if len(args) == 0 || args[0] != "show" {
		return opts, errors.New("config requires subcommand show")
	}
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown config show flag %q", arg)
		}
	}
	return opts, nil
}

func parseModelsArgs(args []string) (commands.ModelsOptions, error) {
	opts := commands.ModelsOptions{}
	for _, arg := range args {
		switch arg {
		case "validate":
			opts.Validate = true
		case "doctor":
			opts.Doctor = true
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown models flag %q", arg)
		}
	}
	return opts, nil
}

func parseRunsArgs(args []string) (commands.RunsOptions, error) {
	opts := commands.RunsOptions{Limit: 20}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--all":
			opts.All = true
		case "--limit":
			if index+1 >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return opts, err
			}
			opts.Limit = value
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return opts, err
				}
				opts.Limit = parsed
				continue
			}
			return opts, fmt.Errorf("unknown runs flag %q", arg)
		}
	}
	if opts.Limit <= 0 {
		return opts, errors.New("--limit must be positive")
	}
	return opts, nil
}

func parseInspectArgs(args []string) (string, commands.InspectOptions, error) {
	opts := commands.InspectOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown inspect flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("inspect requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("inspect requires exactly one run id")
	}
	return runID, opts, nil
}

func parseArtifactsArgs(args []string) (string, commands.ArtifactsOptions, error) {
	opts := commands.ArtifactsOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--type requires a value")
			}
			index++
			opts.Type = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--type="); ok {
				opts.Type = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown artifacts flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("artifacts requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("artifacts requires exactly one run id")
	}
	return runID, opts, nil
}

func parseValidateArgs(args []string) (string, commands.ValidateOptions, error) {
	opts := commands.ValidateOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown validate flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("validate requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("validate requires exactly one run id")
	}
	return runID, opts, nil
}

func parseClipCardsArgs(args []string) (string, commands.ClipCardsOptions, error) {
	opts := commands.ClipCardsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown clip-cards flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("clip-cards requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("clip-cards requires exactly one run id")
	}
	return runID, opts, nil
}

func parseReviewClipsArgs(args []string) (string, commands.ReviewClipsOptions, error) {
	opts := commands.ReviewClipsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-clips flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("review-clips requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("review-clips requires exactly one run id")
	}
	return runID, opts, nil
}

func parseEnhanceRoughcutArgs(args []string) (string, commands.EnhanceRoughcutOptions, error) {
	opts := commands.EnhanceRoughcutOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown enhance-roughcut flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("enhance-roughcut requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("enhance-roughcut requires exactly one run id")
	}
	return runID, opts, nil
}

func parseSelectedClipsArgs(args []string) (string, commands.SelectedClipsOptions, error) {
	opts := commands.SelectedClipsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown selected-clips flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("selected-clips requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("selected-clips requires exactly one run id")
	}
	return runID, opts, nil
}

func parseExportManifestArgs(args []string) (string, commands.ExportManifestOptions, error) {
	opts := commands.ExportManifestOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown export-manifest flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("export-manifest requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("export-manifest requires exactly one run id")
	}
	return runID, opts, nil
}

func parseFFmpegScriptCommandArgs(args []string) (string, commands.FFmpegScriptCommandOptions, error) {
	opts := commands.FFmpegScriptCommandOptions{Mode: "stream-copy"}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--mode":
			if index+1 >= len(args) {
				return "", opts, errors.New("--mode requires a value")
			}
			index++
			opts.Mode = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--mode="); ok {
				opts.Mode = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown ffmpeg-script flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("ffmpeg-script requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("ffmpeg-script requires exactly one run id")
	}
	if opts.Mode != "stream-copy" && opts.Mode != "reencode" {
		return "", opts, fmt.Errorf("unsupported ffmpeg mode %q; supported values: stream-copy, reencode", opts.Mode)
	}
	return runID, opts, nil
}

func parseConcatPlanArgs(args []string) (string, commands.ConcatPlanOptions, error) {
	opts := commands.ConcatPlanOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown concat-plan flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("concat-plan requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("concat-plan requires exactly one run id")
	}
	return runID, opts, nil
}

func parseInspectMaskArgs(args []string) (string, commands.InspectMaskOptions, error) {
	opts := commands.InspectMaskOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown inspect-mask flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("inspect-mask requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("inspect-mask requires exactly one run id")
	}
	return runID, opts, nil
}

func parseMaskPlanArgs(args []string) (string, commands.MaskPlanOptions, error) {
	opts := commands.MaskPlanOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--intent":
			if index+1 >= len(args) {
				return "", opts, errors.New("--intent requires a value")
			}
			index++
			opts.Intent = args[index]
		case "--tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--tone requires a value")
			}
			index++
			opts.Tone = args[index]
		case "--max-caption-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-caption-words requires a value")
			}
			index++
			value, err := parseIntFlag("--max-caption-words", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.MaxCaptionWords = value
		case "--top-k":
			if index+1 >= len(args) {
				return "", opts, errors.New("--top-k requires a value")
			}
			index++
			value, err := parseIntFlag("--top-k", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.TopK = value
		case "--overwrite":
			opts.Overwrite = true
		default:
			if value, ok := strings.CutPrefix(arg, "--intent="); ok {
				opts.Intent = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--tone="); ok {
				opts.Tone = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--max-caption-words="); ok {
				parsed, err := parseIntFlag("--max-caption-words", value)
				if err != nil {
					return "", opts, err
				}
				opts.MaxCaptionWords = parsed
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--top-k="); ok {
				parsed, err := parseIntFlag("--top-k", value)
				if err != nil {
					return "", opts, err
				}
				opts.TopK = parsed
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown mask-plan flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("mask-plan requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("mask-plan requires exactly one run id")
	}
	if opts.MaxCaptionWords < 0 {
		return "", opts, errors.New("--max-caption-words must be positive")
	}
	if opts.TopK < 0 {
		return "", opts, errors.New("--top-k must be positive")
	}
	return runID, opts, nil
}

func parseReviewMaskArgs(args []string) (string, commands.ReviewMaskOptions, error) {
	opts := commands.ReviewMaskOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-mask flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("review-mask requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("review-mask requires exactly one run id")
	}
	return runID, opts, nil
}

func parseMaskValidateArgs(args []string) (string, commands.MaskValidateOptions, error) {
	opts := commands.MaskValidateOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown mask-validate flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("mask-validate requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("mask-validate requires exactly one run id")
	}
	return runID, opts, nil
}

func parseExpansionPlanArgs(args []string) (string, commands.ExpansionPlanOptions, error) {
	opts := commands.ExpansionPlanOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--caption-variants":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-variants requires a value")
			}
			index++
			value, err := parseIntFlag("--caption-variants", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.CaptionVariants = value
		case "--label-max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--label-max-words requires a value")
			}
			index++
			value, err := parseIntFlag("--label-max-words", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.LabelMaxWords = value
		case "--description-max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--description-max-words requires a value")
			}
			index++
			value, err := parseIntFlag("--description-max-words", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.DescriptionMaxWords = value
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expansion-plan flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expansion-plan requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expansion-plan requires exactly one run id")
	}
	return runID, opts, nil
}

func parseVerificationPlanArgs(args []string) (string, commands.VerificationPlanOptions, error) {
	opts := commands.VerificationPlanOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown verification-plan flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("verification-plan requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("verification-plan requires exactly one run id")
	}
	return runID, opts, nil
}

func parseRoutesPlanArgs(args []string) (string, commands.RoutesPlanOptions, error) {
	opts := commands.RoutesPlanOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		case "--strict":
			opts.Strict = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown routes-plan flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("routes-plan requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("routes-plan requires exactly one run id")
	}
	return runID, opts, nil
}

func parseReviseMaskArgs(args []string) (string, commands.ReviseMaskOptions, error) {
	opts := commands.ReviseMaskOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		case "--show-diff":
			opts.ShowDiff = true
		case "--request":
			if index+1 >= len(args) {
				return "", opts, errors.New("--request requires a value")
			}
			index++
			opts.Request = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--request="); ok {
				opts.Request = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown revise-mask flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("revise-mask requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("revise-mask requires exactly one run id")
	}
	if opts.Request == "" {
		return "", opts, errors.New("--request is required")
	}
	return runID, opts, nil
}

func parseMaskSnapshotsArgs(args []string) (string, commands.MaskSnapshotsOptions, error) {
	opts := commands.MaskSnapshotsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown mask-snapshots flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("mask-snapshots requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("mask-snapshots requires exactly one run id")
	}
	return runID, opts, nil
}

func parseInspectMaskSnapshotArgs(args []string) (string, string, commands.InspectMaskSnapshotOptions, error) {
	opts := commands.InspectMaskSnapshotOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown inspect-mask-snapshot flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("inspect-mask-snapshot requires a run id and snapshot id")
	}
	return ids[0], ids[1], opts, nil
}

func parseDiffMaskArgs(args []string) (string, string, commands.DiffMaskOptions, error) {
	opts := commands.DiffMaskOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown diff-mask flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("diff-mask requires a run id and snapshot id")
	}
	return ids[0], ids[1], opts, nil
}

func parseOpenReportArgs(args []string) (string, bool, error) {
	runID := ""
	open := false
	for _, arg := range args {
		switch arg {
		case "--open":
			open = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", false, fmt.Errorf("unknown open-report flag %q", arg)
			}
			if runID != "" {
				return "", false, errors.New("open-report requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", false, errors.New("open-report requires exactly one run id")
	}
	return runID, open, nil
}

func parsePipelineArgs(args []string) (string, commands.RunOptions, error) {
	preset := ""
	forwarded := []string{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--preset":
			if index+1 >= len(args) {
				return "", commands.RunOptions{}, errors.New("--preset requires a value")
			}
			index++
			preset = args[index]
		case strings.HasPrefix(arg, "--preset="):
			preset = strings.TrimPrefix(arg, "--preset=")
		default:
			forwarded = append(forwarded, arg)
		}
	}
	if preset == "" {
		return "", commands.RunOptions{}, errors.New("pipeline requires --preset")
	}
	base, err := presetRunOptions(preset)
	if err != nil {
		return "", commands.RunOptions{}, err
	}
	return parseRunArgsWithBase(forwarded, base)
}

func parseBatchArgs(args []string) (string, commands.BatchOptions, error) {
	opts := commands.BatchOptions{Preset: "shorts"}
	inputDir := ""
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--recursive":
			opts.Recursive = true
		case "--fail-fast":
			opts.FailFast = true
		case "--dry-run":
			opts.DryRun = true
		case "--validate":
			opts.Validate = true
		case "--export":
			opts.Export = true
		case "--export-and-validate":
			opts.ExportAndValidate = true
			opts.Export = true
			opts.Validate = true
		case "--limit":
			if index+1 >= len(args) {
				return "", opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.Limit = value
			limitSet = true
		case "--preset":
			if index+1 >= len(args) {
				return "", opts, errors.New("--preset requires a value")
			}
			index++
			opts.Preset = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return "", opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown batch flag %q", arg)
			}
			if inputDir != "" {
				return "", opts, errors.New("batch requires exactly one input directory")
			}
			inputDir = arg
		}
	}
	if inputDir == "" {
		return "", opts, errors.New("batch requires exactly one input directory")
	}
	if limitSet && opts.Limit <= 0 {
		return "", opts, errors.New("--limit must be positive")
	}
	runOpts, err := presetRunOptions(opts.Preset)
	if err != nil {
		return "", opts, err
	}
	opts.RunOptions = runOpts
	return inputDir, opts, nil
}

func parseInspectBatchArgs(args []string) (string, commands.InspectBatchOptions, error) {
	opts := commands.InspectBatchOptions{}
	batchID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown inspect-batch flag %q", arg)
			}
			if batchID != "" {
				return "", opts, errors.New("inspect-batch requires exactly one batch id")
			}
			batchID = arg
		}
	}
	if batchID == "" {
		return "", opts, errors.New("inspect-batch requires exactly one batch id")
	}
	return batchID, opts, nil
}

func parseWatchArgs(args []string) (string, commands.WatchOptions, error) {
	opts := commands.WatchOptions{Preset: "shorts", IntervalSeconds: 5}
	inputDir := ""
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--recursive":
			opts.Recursive = true
		case "--once":
			opts.Once = true
		case "--fail-fast":
			opts.FailFast = true
		case "--validate":
			opts.Validate = true
		case "--export":
			opts.Export = true
		case "--export-and-validate":
			opts.ExportAndValidate = true
			opts.Export = true
			opts.Validate = true
		case "--ignore-registry":
			opts.IgnoreRegistry = true
		case "--limit":
			if index+1 >= len(args) {
				return "", opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.Limit = value
			limitSet = true
		case "--interval-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--interval-seconds requires a value")
			}
			index++
			value, err := parseIntFlag("--interval-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.IntervalSeconds = value
		case "--preset":
			if index+1 >= len(args) {
				return "", opts, errors.New("--preset requires a value")
			}
			index++
			opts.Preset = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return "", opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--interval-seconds="); ok {
				parsed, err := parseIntFlag("--interval-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.IntervalSeconds = parsed
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown watch flag %q", arg)
			}
			if inputDir != "" {
				return "", opts, errors.New("watch requires exactly one input directory")
			}
			inputDir = arg
		}
	}
	if inputDir == "" {
		return "", opts, errors.New("watch requires exactly one input directory")
	}
	if limitSet && opts.Limit <= 0 {
		return "", opts, errors.New("--limit must be positive")
	}
	if opts.IntervalSeconds <= 0 {
		return "", opts, errors.New("--interval-seconds must be positive")
	}
	runOpts, err := presetRunOptions(opts.Preset)
	if err != nil {
		return "", opts, err
	}
	opts.RunOptions = runOpts
	return inputDir, opts, nil
}

func parseWatchStatusArgs(args []string) (commands.WatchStatusOptions, error) {
	opts := commands.WatchStatusOptions{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown watch-status flag %q", arg)
		}
	}
	return opts, nil
}

func parseRetryBatchArgs(args []string) (string, commands.RetryBatchOptions, error) {
	opts := commands.RetryBatchOptions{}
	id := ""
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--fail-fast":
			opts.FailFast = true
		case "--dry-run":
			opts.DryRun = true
		case "--validate":
			opts.Validate = true
		case "--export":
			opts.Export = true
		case "--export-and-validate":
			opts.ExportAndValidate = true
			opts.Export = true
			opts.Validate = true
		case "--limit":
			if index+1 >= len(args) {
				return "", opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.Limit = value
			limitSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return "", opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown retry-batch flag %q", arg)
			}
			if id != "" {
				return "", opts, errors.New("retry-batch requires exactly one batch id")
			}
			id = arg
		}
	}
	if id == "" {
		return "", opts, errors.New("retry-batch requires exactly one batch id")
	}
	if limitSet && opts.Limit <= 0 {
		return "", opts, errors.New("--limit must be positive")
	}
	return id, opts, nil
}

func parseRetryWatchArgs(args []string) (commands.RetryWatchOptions, error) {
	opts := commands.RetryWatchOptions{Preset: "shorts"}
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--fail-fast":
			opts.FailFast = true
		case "--dry-run":
			opts.DryRun = true
		case "--validate":
			opts.Validate = true
		case "--export":
			opts.Export = true
		case "--export-and-validate":
			opts.ExportAndValidate = true
			opts.Export = true
			opts.Validate = true
		case "--limit":
			if index+1 >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return opts, err
			}
			opts.Limit = value
			limitSet = true
		case "--preset":
			if index+1 >= len(args) {
				return opts, errors.New("--preset requires a value")
			}
			index++
			opts.Preset = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				continue
			}
			return opts, fmt.Errorf("unknown retry-watch flag %q", arg)
		}
	}
	if limitSet && opts.Limit <= 0 {
		return opts, errors.New("--limit must be positive")
	}
	runOpts, err := presetRunOptions(opts.Preset)
	if err != nil {
		return opts, err
	}
	opts.RunOptions = runOpts
	return opts, nil
}

func parseRerunArgs(args []string) (string, commands.RerunOptions, error) {
	opts := commands.RerunOptions{}
	id := ""
	presetSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--validate":
			opts.Validate = true
		case "--export":
			opts.Export = true
		case "--export-and-validate":
			opts.ExportAndValidate = true
			opts.Export = true
			opts.Validate = true
		case "--preset":
			if index+1 >= len(args) {
				return "", opts, errors.New("--preset requires a value")
			}
			index++
			opts.Preset = args[index]
			presetSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				presetSet = true
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown rerun flag %q", arg)
			}
			if id != "" {
				return "", opts, errors.New("rerun requires exactly one run id")
			}
			id = arg
		}
	}
	if id == "" {
		return "", opts, errors.New("rerun requires exactly one run id")
	}
	if presetSet {
		runOpts, err := presetRunOptions(opts.Preset)
		if err != nil {
			return "", opts, err
		}
		opts.RunOptions = runOpts
		opts.PresetOverride = true
	}
	return id, opts, nil
}

func parseCleanupArgs(args []string) (commands.CleanupOptions, error) {
	opts := commands.CleanupOptions{}
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--failed":
			opts.Failed = true
		case "--stale-running":
			opts.StaleRunning = true
		case "--missing-manifest":
			opts.MissingManifest = true
		case "--delete":
			opts.Delete = true
		case "--json":
			opts.JSON = true
		case "--yes":
			opts.Yes = true
		case "--limit":
			if index+1 >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return opts, err
			}
			opts.Limit = value
			limitSet = true
		case "--older-than-hours":
			if index+1 >= len(args) {
				return opts, errors.New("--older-than-hours requires a value")
			}
			index++
			value, err := parseIntFlag("--older-than-hours", args[index])
			if err != nil {
				return opts, err
			}
			opts.OlderThanHours = value
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--older-than-hours="); ok {
				parsed, err := parseIntFlag("--older-than-hours", value)
				if err != nil {
					return opts, err
				}
				opts.OlderThanHours = parsed
				continue
			}
			return opts, fmt.Errorf("unknown cleanup flag %q", arg)
		}
	}
	if limitSet && opts.Limit <= 0 {
		return opts, errors.New("--limit must be positive")
	}
	if opts.OlderThanHours < 0 {
		return opts, errors.New("--older-than-hours must be non-negative")
	}
	return opts, nil
}

func parsePlanArgs(args []string) (string, commands.PlanOptions, error) {
	opts := commands.PlanOptions{}
	inputFile := ""
	trailingGoal := []string{}
	limitSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--execute":
			opts.Execute = true
		case "--dry-run":
			opts.DryRun = true
		case "--with-export":
			opts.WithExport = true
		case "--with-validate":
			opts.WithValidate = true
		case "--with-report":
			opts.WithReport = true
			opts.WithReportSet = true
		case "--recursive":
			opts.Recursive = true
		case "--once":
			opts.Once = true
		case "--goal":
			if index+1 >= len(args) {
				return "", opts, errors.New("--goal requires a value")
			}
			index++
			opts.Goal = args[index]
		case "--preset":
			if index+1 >= len(args) {
				return "", opts, errors.New("--preset requires a value")
			}
			index++
			opts.Preset = args[index]
		case "--mode":
			if index+1 >= len(args) {
				return "", opts, errors.New("--mode requires a value")
			}
			index++
			opts.Mode = args[index]
		case "--max-clips":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-clips requires a value")
			}
			index++
			value, err := parseIntFlag("--max-clips", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.MaxClips = value
		case "--limit":
			if index+1 >= len(args) {
				return "", opts, errors.New("--limit requires a value")
			}
			index++
			value, err := parseIntFlag("--limit", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.Limit = value
			limitSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				opts.Goal = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--mode="); ok {
				opts.Mode = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--max-clips="); ok {
				parsed, err := parseIntFlag("--max-clips", value)
				if err != nil {
					return "", opts, err
				}
				opts.MaxClips = parsed
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				parsed, err := parseIntFlag("--limit", value)
				if err != nil {
					return "", opts, err
				}
				opts.Limit = parsed
				limitSet = true
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown plan flag %q", arg)
			}
			if inputFile == "" {
				inputFile = arg
			} else {
				trailingGoal = append(trailingGoal, arg)
			}
		}
	}
	if inputFile == "" {
		return "", opts, errors.New("plan requires exactly one input file")
	}
	if opts.Goal == "" && len(trailingGoal) > 0 {
		opts.Goal = strings.Join(trailingGoal, " ")
	}
	if opts.Goal == "" {
		return "", opts, errors.New("--goal is required")
	}
	if opts.MaxClips < 0 {
		return "", opts, errors.New("--max-clips must be positive")
	}
	if limitSet && opts.Limit <= 0 {
		return "", opts, errors.New("--limit must be positive")
	}
	if opts.Mode != "" && opts.Mode != "file" && opts.Mode != "batch" && opts.Mode != "watch" {
		return "", opts, fmt.Errorf("unknown plan mode %q; supported values: file, batch, watch", opts.Mode)
	}
	if opts.Preset != "" {
		if _, err := presetRunOptions(opts.Preset); err != nil {
			return "", opts, err
		}
	}
	return inputFile, opts, nil
}

func parseInspectPlanArgs(args []string) (string, commands.InspectPlanOptions, error) {
	opts := commands.InspectPlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown inspect-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("inspect-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("inspect-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parsePlanArtifactsArgs(args []string) (string, commands.PlanArtifactsOptions, error) {
	opts := commands.PlanArtifactsOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown plan-artifacts flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("plan-artifacts requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("plan-artifacts requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewPlanArgs(args []string) (string, commands.ReviewPlanOptions, error) {
	opts := commands.ReviewPlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseExecutePlanArgs(args []string) (string, commands.ExecutePlanOptions, error) {
	opts := commands.ExecutePlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--dry-run":
			opts.DryRun = true
		case "--with-export":
			opts.WithExport = true
		case "--with-validate":
			opts.WithValidate = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown execute-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("execute-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("execute-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseDiffPlanArgs(args []string) (string, string, commands.DiffPlanOptions, error) {
	opts := commands.DiffPlanOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown diff-plan flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("diff-plan requires exactly two plan ids")
	}
	return ids[0], ids[1], opts, nil
}

func parseRevisePlanArgs(args []string) (string, commands.RevisePlanOptions, error) {
	opts := commands.RevisePlanOptions{}
	planID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		case "--show-diff":
			opts.ShowDiff = true
		case "--request":
			if index+1 >= len(args) {
				return "", opts, errors.New("--request requires a value")
			}
			index++
			opts.Request = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--request="); ok {
				opts.Request = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown revise-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("revise-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("revise-plan requires exactly one plan id")
	}
	if opts.Request == "" {
		return "", opts, errors.New("--request is required")
	}
	return planID, opts, nil
}

func parseInspectSnapshotArgs(args []string) (string, string, commands.InspectSnapshotOptions, error) {
	opts := commands.InspectSnapshotOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown inspect-snapshot flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("inspect-snapshot requires a plan id and snapshot id")
	}
	return ids[0], ids[1], opts, nil
}

func parseDiffSnapshotArgs(args []string) (string, string, commands.DiffSnapshotOptions, error) {
	opts := commands.DiffSnapshotOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown diff-snapshot flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("diff-snapshot requires a plan id and snapshot id")
	}
	return ids[0], ids[1], opts, nil
}

func presetRunOptions(preset string) (commands.RunOptions, error) {
	switch preset {
	case "shorts":
		opts, err := configuredRunOptions(true)
		if err != nil {
			return commands.RunOptions{}, err
		}
		opts.WithTranscript = true
		opts.WithCaptions = true
		opts.WithChunks = true
		opts.WithHighlights = true
		opts.WithRoughcut = true
		opts.WithFFmpegScript = true
		opts.WithReport = true
		return opts, nil
	case "metadata":
		opts, err := configuredRunOptions(false)
		if err != nil {
			return commands.RunOptions{}, err
		}
		return opts, nil
	default:
		return commands.RunOptions{}, fmt.Errorf("unknown pipeline preset %q; supported values: shorts, metadata", preset)
	}
}

func configuredRunOptions(applyEnabled bool) (commands.RunOptions, error) {
	opts := defaultRunOptions()
	if _, err := os.Stat(config.DefaultPath); err != nil {
		if os.IsNotExist(err) {
			return opts, nil
		}
		return opts, fmt.Errorf("stat config: %w", err)
	}
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		return opts, err
	}
	applyConfig(&opts, cfg, applyEnabled)
	return opts, nil
}

func applyConfig(opts *commands.RunOptions, cfg config.Config, applyEnabled bool) {
	if cfg.Python.Interpreter != "" {
		opts.PythonInterpreter = cfg.Python.Interpreter
	}
	if cfg.Transcription.ModelSize != "" {
		opts.TranscriptModelSize = cfg.Transcription.ModelSize
	}
	if cfg.Chunks.TargetSeconds != 0 {
		opts.ChunkTargetSeconds = cfg.Chunks.TargetSeconds
	}
	if cfg.Chunks.MaxGapSeconds != 0 {
		opts.ChunkMaxGapSeconds = cfg.Chunks.MaxGapSeconds
	}
	if cfg.Highlights.TopK != 0 {
		opts.HighlightTopK = cfg.Highlights.TopK
	}
	if cfg.Highlights.MinDurationSeconds != 0 {
		opts.HighlightMinDuration = cfg.Highlights.MinDurationSeconds
	}
	if cfg.Highlights.MaxDurationSeconds != 0 {
		opts.HighlightMaxDuration = cfg.Highlights.MaxDurationSeconds
	}
	if cfg.Roughcut.MaxClips != 0 {
		opts.RoughcutMaxClips = cfg.Roughcut.MaxClips
	}
	if cfg.FFmpegScript.OutputFormat != "" {
		opts.FFmpegOutputFormat = cfg.FFmpegScript.OutputFormat
	}
	if cfg.FFmpegScript.Mode != "" {
		opts.FFmpegMode = cfg.FFmpegScript.Mode
	}
	if !applyEnabled {
		return
	}
	opts.WithTranscript = cfg.Transcription.Enabled
	opts.WithCaptions = cfg.Captions.Enabled
	opts.WithChunks = cfg.Chunks.Enabled
	opts.WithHighlights = cfg.Highlights.Enabled
	opts.WithRoughcut = cfg.Roughcut.Enabled
	opts.WithFFmpegScript = cfg.FFmpegScript.Enabled
	opts.WithReport = cfg.Report.Enabled
}

func parseMaskDecisionsArgs(args []string) (string, commands.MaskDecisionsOptions, error) {
	opts := commands.MaskDecisionsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown mask-decisions flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("mask-decisions requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("mask-decisions requires exactly one run id")
	}
	return runID, opts, nil
}

func parseMaskDecisionArgs(args []string) (string, string, commands.MaskDecisionOptions, error) {
	opts := commands.MaskDecisionOptions{}
	ids := []string{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		case "--set":
			if index+1 >= len(args) {
				return "", "", opts, errors.New("--set requires a value")
			}
			index++
			opts.Set = args[index]
		case "--reason":
			if index+1 >= len(args) {
				return "", "", opts, errors.New("--reason requires a value")
			}
			index++
			opts.Reason = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--set="); ok {
				opts.Set = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--reason="); ok {
				opts.Reason = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown mask-decision flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("mask-decision requires a run id and decision id")
	}
	if opts.Set == "" {
		return "", "", opts, errors.New("--set is required")
	}
	return ids[0], ids[1], opts, nil
}

func parseMaskRemoveDecisionArgs(args []string) (string, string, commands.MaskRemoveDecisionOptions, error) {
	opts := commands.MaskRemoveDecisionOptions{}
	ids := []string{}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", "", opts, fmt.Errorf("unknown mask-remove-decision flag %q", arg)
			}
			ids = append(ids, arg)
		}
	}
	if len(ids) != 2 {
		return "", "", opts, errors.New("mask-remove-decision requires a run id and decision id")
	}
	return ids[0], ids[1], opts, nil
}

func parseMaskReorderArgs(args []string) (string, commands.MaskReorderOptions, error) {
	opts := commands.MaskReorderOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		case "--order":
			if index+1 >= len(args) {
				return "", opts, errors.New("--order requires a value")
			}
			index++
			opts.Order = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--order="); ok {
				opts.Order = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown mask-reorder flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("mask-reorder requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("mask-reorder requires exactly one run id")
	}
	if opts.Order == "" {
		return "", opts, errors.New("--order is required")
	}
	return runID, opts, nil
}

func parseVerifyExpansionsArgs(args []string) (string, commands.VerifyExpansionsOptions, error) {
	opts := commands.VerifyExpansionsOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--tolerance-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--tolerance-seconds requires a value")
			}
			index++
			v, err := parseFloatFlag("--tolerance-seconds", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.ToleranceSeconds = v
		default:
			if value, ok := strings.CutPrefix(arg, "--tolerance-seconds="); ok {
				v, err := parseFloatFlag("--tolerance-seconds", value)
				if err != nil {
					return "", opts, err
				}
				opts.ToleranceSeconds = v
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown verify-expansions flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("verify-expansions requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("verify-expansions requires exactly one run id")
	}
	if opts.ToleranceSeconds < 0 {
		return "", opts, errors.New("--tolerance-seconds must be non-negative")
	}
	return runID, opts, nil
}

func parseReviewVerificationArgs(args []string) (string, commands.ReviewVerificationOptions, error) {
	opts := commands.ReviewVerificationOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-verification flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("review-verification requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("review-verification requires exactly one run id")
	}
	return runID, opts, nil
}

func parseExpandStubArgs(args []string) (string, commands.ExpandStubOptions, error) {
	opts := commands.ExpandStubOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--task-type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--task-type requires a value")
			}
			index++
			opts.TaskType = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--task-type="); ok {
				opts.TaskType = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expand-stub flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expand-stub requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expand-stub requires exactly one run id")
	}
	if opts.TaskType != "" {
		switch opts.TaskType {
		case "caption_variants", "timeline_labels", "short_descriptions":
		default:
			return "", opts, fmt.Errorf("unknown --task-type %q; supported: caption_variants, timeline_labels, short_descriptions", opts.TaskType)
		}
	}
	return runID, opts, nil
}

func parseExpandDryRunArgs(args []string) (string, commands.ExpandDryRunOptions, error) {
	opts := commands.ExpandDryRunOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		case "--task-type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--task-type requires a value")
			}
			index++
			opts.TaskType = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--task-type="); ok {
				opts.TaskType = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expand-dry-run flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expand-dry-run requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expand-dry-run requires exactly one run id")
	}
	if opts.TaskType != "" {
		switch opts.TaskType {
		case "caption_variants", "timeline_labels", "short_descriptions":
		default:
			return "", opts, fmt.Errorf("unknown --task-type %q; supported: caption_variants, timeline_labels, short_descriptions", opts.TaskType)
		}
	}
	return runID, opts, nil
}

func parseExpandLocalStubArgs(args []string) (string, commands.ExpandLocalStubOptions, error) {
	opts := commands.ExpandLocalStubOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--task-type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--task-type requires a value")
			}
			index++
			opts.TaskType = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--task-type="); ok {
				opts.TaskType = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expand-local-stub flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expand-local-stub requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expand-local-stub requires exactly one run id")
	}
	if opts.TaskType != "" {
		switch opts.TaskType {
		case "caption_variants", "timeline_labels", "short_descriptions":
		default:
			return "", opts, fmt.Errorf("unknown --task-type %q; supported: caption_variants, timeline_labels, short_descriptions", opts.TaskType)
		}
	}
	return runID, opts, nil
}

func parseExpandArgs(args []string) (string, commands.ExpandOptions, error) {
	opts := commands.ExpandOptions{}
	runID := ""
	maxTasksSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		case "--dry-run":
			opts.DryRun = true
		case "--fail-fast":
			opts.FailFast = true
		case "--task-type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--task-type requires a value")
			}
			index++
			opts.TaskType = args[index]
		case "--max-tasks":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-tasks requires a value")
			}
			index++
			value, err := parseIntFlag("--max-tasks", args[index])
			if err != nil {
				return "", opts, err
			}
			opts.MaxTasks = value
			maxTasksSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--task-type="); ok {
				opts.TaskType = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--max-tasks="); ok {
				parsed, err := parseIntFlag("--max-tasks", value)
				if err != nil {
					return "", opts, err
				}
				opts.MaxTasks = parsed
				maxTasksSet = true
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expand flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expand requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expand requires exactly one run id")
	}
	if opts.TaskType != "" {
		switch opts.TaskType {
		case "caption_variants", "timeline_labels", "short_descriptions":
		default:
			return "", opts, fmt.Errorf("unknown --task-type %q; supported: caption_variants, timeline_labels, short_descriptions", opts.TaskType)
		}
	}
	if maxTasksSet && opts.MaxTasks <= 0 {
		return "", opts, errors.New("--max-tasks must be positive")
	}
	return runID, opts, nil
}

func parseReviewModelRequestsArgs(args []string) (string, commands.ReviewModelRequestsOptions, error) {
	opts := commands.ReviewModelRequestsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-model-requests flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("review-model-requests requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("review-model-requests requires exactly one run id")
	}
	return runID, opts, nil
}

func parseExpansionValidateArgs(args []string) (string, commands.ExpansionValidateOptions, error) {
	opts := commands.ExpansionValidateOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown expansion-validate flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("expansion-validate requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("expansion-validate requires exactly one run id")
	}
	return runID, opts, nil
}

func parseReviewExpansionsArgs(args []string) (string, commands.ReviewExpansionsOptions, error) {
	opts := commands.ReviewExpansionsOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-expansions flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("review-expansions requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("review-expansions requires exactly one run id")
	}
	return runID, opts, nil
}

func parseRoutePreviewArgs(args []string) (string, commands.RoutePreviewOptions, error) {
	opts := commands.RoutePreviewOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown route-preview flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("route-preview requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("route-preview requires exactly one run id")
	}
	return runID, opts, nil
}

func parseFloatFlag(name string, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	return parsed, nil
}

func parseIntFlag(name string, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}
