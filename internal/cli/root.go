package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mirelahmd/byom-video/internal/commands"
	"github.com/mirelahmd/byom-video/internal/config"
)

const usage = `byom-video is a local-first media/video workflow control plane.

Usage:
  byom-video doctor [--transcription] [--media]
  byom-video version
  byom-video init [--force]
  byom-video config show [--json]
  byom-video models [--json]
  byom-video models validate [--json]
  byom-video models doctor [--json]
  byom-video tools [--json]
  byom-video tools validate [--json] [--strict] [--check-env]
  byom-video tools requirements --goal <text> [--json]
  byom-video creative-plan <input-file> --goal <text> [--json] [--strict] [--write-artifact]
  byom-video creative-plans
  byom-video inspect-creative-plan <creative_plan_id> [--json]
  byom-video review-creative-plan <creative_plan_id> [--json] [--write-artifact]
  byom-video approve-creative-plan <creative_plan_id>
  byom-video creative-plan-events <creative_plan_id> [--json]
  byom-video creative-preview <creative_plan_id> [--json] [--strict] [--overwrite] [--check-env]
  byom-video execute-creative-plan <creative_plan_id> [--yes] [--dry-run] [--strict] [--check-env] [--json]
  byom-video creative-execute-stub <creative_plan_id> [--yes] [--overwrite] [--json] [--step-type <type>] [--dry-run]
  byom-video review-creative-outputs <creative_plan_id> [--json] [--write-artifact]
  byom-video creative-timeline <creative_plan_id> [--run-id <run_id>] [--overwrite] [--json] [--prefer-goal]
  byom-video creative-render-plan <creative_plan_id> [--overwrite] [--json]
  byom-video review-creative-timeline <creative_plan_id> [--json] [--write-artifact]
  byom-video creative-assemble <creative_plan_id> [--overwrite] [--json] [--mode <reencode|stream-copy>] [--keep-work] [--dry-run] [--max-clips <n>] [--burn-captions] [--captions <path>] [--allow-missing-captions] [--mix-voiceover] [--voiceover <path>] [--allow-missing-voiceover] [--platform <preset>] [--fit <crop|pad>] [--background <color>] [--caption-position <bottom|center|top|auto>] [--caption-margin <n>] [--caption-style <default|bold|boxed>]
  byom-video validate-creative-assemble <creative_plan_id> [--json]
  byom-video review-creative-assemble <creative_plan_id> [--json] [--write-artifact]
  byom-video creative-result <creative_plan_id> [--json] [--write-artifact]
  byom-video validate-creative-plan <creative_plan_id> [--json]
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
  byom-video plan <path> --goal <text> [--mode <file|batch|watch>] [--goal-aware] [--goal-use-ollama] [--goal-fallback-deterministic] [--execute] [--dry-run] [--preset <shorts|metadata>] [--max-clips <n>] [--recursive] [--once] [--limit <n>] [--with-export] [--with-validate] [--with-report]
  byom-video plans
  byom-video inspect-plan <plan_id> [--json]
  byom-video plan-artifacts <plan_id> [--json]
  byom-video review-plan <plan_id> [--json] [--write-artifact]
  byom-video approve-plan <plan_id>
  byom-video execute-plan <plan_id> [--yes] [--dry-run]
  byom-video agent-result <plan_id> [--json] [--write-artifact]
  byom-video diff-plan <plan_id_a> <plan_id_b> [--json] [--write-artifact]
  byom-video revise-plan <plan_id> --request <text> [--dry-run] [--json] [--show-diff]
  byom-video snapshots <plan_id>
  byom-video inspect-snapshot <plan_id> <snapshot_id> [--json]
  byom-video diff-snapshot <plan_id> <snapshot_id> [--json] [--write-artifact]
  byom-video runs [--limit <n> | --all]
  byom-video inspect <run_id> [--json]
  byom-video artifacts <run_id> [--type <name>]
  byom-video validate <run_id> [--json]
  byom-video clip-cards <run_id> [--overwrite] [--json] [--prefer-goal-roughcut]
  byom-video review-clips <run_id> [--json] [--write-artifact]
  byom-video enhance-roughcut <run_id> [--overwrite] [--json]
  byom-video selected-clips <run_id> [--overwrite] [--json] [--prefer-goal-roughcut]
  byom-video export-manifest <run_id> [--overwrite] [--json]
  byom-video ffmpeg-script <run_id> [--mode <stream-copy|reencode>] [--overwrite] [--json]
  byom-video concat-plan <run_id> [--overwrite] [--json]
  byom-video goal-handoff <run_id> [--overwrite] [--json]
  byom-video goal-review-bundle <run_id> [--json] [--overwrite]
  byom-video goal-rerank <run_id> --goal <text> [--use-ollama] [--fallback-deterministic]
  byom-video goal-roughcut <run_id> [--overwrite] [--json]
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
  byom-video style init [--force] [--style-dir <path>] [--json]
  byom-video style inspect [--style-dir <path>] [--json]
  byom-video style validate [--style-dir <path>] [--strict] [--json]
  byom-video creative-generate-script <creative_plan_id> [--overwrite] [--json] [--style-dir <path>] [--no-style] [--route <name>] [--model <name>] [--fallback-stub] [--max-words <n>] [--tone <text>]
  byom-video review-script <creative_plan_id> [--json] [--write-artifact]
  byom-video creative-caption-variants <creative_plan_id> [--overwrite] [--json] [--style-dir <path>] [--no-style] [--route <name>] [--model <name>] [--fallback-stub] [--count <n>] [--max-words <n>] [--tone <text>]
  byom-video review-caption-variants <creative_plan_id> [--json] [--write-artifact]
  byom-video creative-voiceover-text <creative_plan_id> [--overwrite] [--json] [--max-words <n>] [--tone <text>] [--source <auto|script|goal>]
  byom-video voiceover-status <creative_plan_id> [--json]
  byom-video review-voiceover <creative_plan_id> [--json] [--write-artifact]
  byom-video validate-voiceover <creative_plan_id> [--json] [--require-audio]
  byom-video creative-generate-voiceover <creative_plan_id> [--overwrite] [--json] [--dry-run] [--check-env] [--prepare-text] [--route <name>] [--backend <name>] [--voice-id <id>] [--model <model>] [--timeout-seconds <n>] [--text <text>] [--stability <0.0-1.0>] [--similarity-boost <0.0-1.0>] [--output-format <format>]
  byom-video review-generated-voiceover <creative_plan_id> [--json] [--write-artifact]
  byom-video make [<input-file>] --goal <text> [--yes] [--dry-run] [--preset <shorts|metadata>] [--skip-pipeline <run_id>] [--strict-input] [--export] [--require-export] [--generate-script] [--script-fallback-stub] [--style-dir <path>] [--no-style] [--script-route <name>] [--script-model <name>] [--script-max-words <n>] [--script-tone <text>] [--generate-captions] [--caption-fallback-stub] [--caption-count <n>] [--caption-max-words <n>] [--caption-tone <text>] [--prepare-voiceover] [--voiceover-max-words <n>] [--voiceover-tone <text>] [--require-voiceover] [--generate-voiceover] [--voiceover-route <name>] [--voiceover-backend <name>] [--voiceover-voice-id <id>] [--voiceover-timeout-seconds <n>] [--voiceover-dry-run] [--voiceover-check-env] [--allow-missing-generated-voiceover] [--voiceover-stability <0.0-1.0>] [--voiceover-similarity-boost <0.0-1.0>] [--voiceover-output-format <format>] [--platform <preset>] [--fit <crop|pad>] [--background <color>] [--caption-position <bottom|center|top|auto>] [--caption-margin <n>] [--caption-style <default|bold|boxed>] [--burn-captions] [--allow-missing-captions] [--mix-voiceover] [--voiceover <path>] [--allow-missing-voiceover] [--mode <reencode|stream-copy>] [--goal-aware] [--use-ollama-goal] [--keep-work] [--overwrite] [--json]
  byom-video makes [--json]
  byom-video inspect-make <make_id> [--json]
  byom-video make-result <make_id> [--json] [--write-artifact]
  byom-video revise-make <make_id> --request <text> [--dry-run] [--json] [--yes] [--overwrite] [--reassemble] [--validate] [--allow-provider-calls] [--fallback-stub]
  byom-video make-revisions <make_id> [--json]
  byom-video inspect-make-revision <make_id> <revision_id> [--json]
  byom-video review-make-revision <make_id> <revision_id> [--json] [--write-artifact]
  byom-video job-create --type <make|revise_make|validate_creative_assemble> [--goal <text>] [--preset <name>] [--make-id <id>] [--request <text>] [--reassemble] [--plan-id <id>] [--allow-provider-calls] [--allow-overwrite] [--allow-external-network] [--json]
  byom-video jobs [--json] [--filter <status>] [--limit <n>]
  byom-video job-inspect <job_id> [--json]
  byom-video job-events <job_id> [--json] [--limit <n>]
  byom-video job-approve <job_id> [--json]
  byom-video job-reject <job_id> [--reason <text>] [--json]
  byom-video job-cancel <job_id> [--reason <text>] [--json]
  byom-video job-run <job_id> [--yes] [--json]
  byom-video job-result <job_id> [--json]
  byom-video job-validate <job_id> [--json]
  byom-video job-worker [--once | --loop | --status] [--interval <duration>] [--max-jobs <n>] [--json] [--dry-run] [--allow-provider-calls] [--allow-overwrite] [--fail-fast] [--force-lock]
  byom-video daemon start [--interval <duration>] [--max-jobs <n>] [--allow-provider-calls] [--allow-overwrite] [--fail-fast] [--force] [--reset-log] [--json]
  byom-video daemon stop [--force] [--json]
  byom-video daemon status [--json]
  byom-video daemon logs [--lines <n>] [--json]
  byom-video queue [--json] [--limit <n>] [--failed] [--approval-needed] [--running]
  byom-video queue health [--json] [--strict] [--stale-after <duration>] [--write-report]
  byom-video agent-planner-diagnose [--planner <deterministic|ollama>] [--planner-model <name>] [--planner-backend <url>] [--planner-route <key>] [--planner-timeout-seconds <n>] [--check] [--json]
  byom-video agent-plan --goal <text> [--input <video_path>] [--make-id <make_id>] [--creative-plan-id <plan_id>] [--run-id <run_id>] [--json] [--write-review] [--allow-provider-calls] [--allow-overwrite] [--platform <preset>] [--style-dir <path>] [--dry-run] [--planner <deterministic|ollama>] [--planner-model <name>] [--planner-backend <url>] [--planner-route <key>] [--planner-fallback-deterministic] [--planner-timeout-seconds <n>] [--planner-temperature <float>] [--planner-max-output-chars <n>]
  byom-video agent-plans [--json] [--status <status>] [--limit <n>]
  byom-video inspect-agent-plan <plan_id> [--json]
  byom-video review-agent-plan <plan_id> [--json] [--write-artifact]
  byom-video agent-policy <plan_id> [--json]
  byom-video approve-agent-plan <plan_id> [--json]
  byom-video reject-agent-plan <plan_id> [--reason <text>] [--json]
  byom-video agent-plan-to-job <plan_id> [--dry-run] [--yes] [--json] [--allow-provider-calls] [--allow-overwrite] [--approve-jobs] [--force]
  byom-video agent-plan-jobs <plan_id> [--json]
  byom-video agent-run <plan_id> [--yes] [--convert] [--approve-jobs] [--run-jobs | --worker-once | --start-daemon] [--dry-run] [--json] [--allow-provider-calls] [--allow-overwrite] [--force] [--fail-fast] [--write-summary]
  byom-video agent-graph-run <plan_id> [--json] [--dry-run] [--workers-dir <path>]
  byom-video agent-orchestrate --goal <text> [--input <video_path>] [--json] [--skip-graph] [--workers-dir <path>]
  byom-video visual-requests <agent_plan_id> [--json] [--overwrite]
  byom-video execute-visual-requests <agent_plan_id> --yes --allow-provider-calls --allow-external-network [--json] [--overwrite] [--request-id <id>]
  byom-video review-visual-generation <agent_plan_id> [--json] [--write-artifact]
  byom-video create [<input>] --goal <text> [--json] [--dry-run] [--write-review] [--yes] [--approval-scope <preview|local|provider|full>] [--convert] [--approve-jobs] [--run-jobs | --worker-once | --start-daemon]
  byom-video create-result <create_session_id> [--json] [--write-artifact]
  byom-video create-sessions [--json] [--status <status>] [--limit <n>]
  byom-video inspect-create-session <create_session_id> [--json]
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
		doctorOpts, err := parseDoctorArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Doctor(stdout, doctorOpts); err != nil {
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
	case "tools":
		if len(args) > 1 && args[1] == "requirements" {
			goal, opts, err := parseToolsRequirementsArgs(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			if err := commands.ToolsRequirements(stdout, goal, opts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		opts, validateOpts, err := parseToolsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if validateOpts != nil {
			if err := commands.ToolsValidate(stdout, *validateOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		if err := commands.Tools(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-plan":
		inputPath, opts, err := parseCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativePlanCommand(inputPath, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-plans":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "error: creative-plans does not accept arguments")
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativePlans(stdout); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-creative-plan":
		planID, opts, err := parseInspectCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectCreativePlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-creative-plan":
		planID, opts, err := parseReviewCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewCreativePlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "approve-creative-plan":
		planID, err := parseApproveCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ApproveCreativePlan(planID, stdout, commands.ApproveCreativePlanOptions{}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-plan-events":
		planID, opts, err := parseCreativePlanEventsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativePlanEvents(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-preview":
		planID, opts, err := parseCreativePreviewArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativePreview(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "execute-creative-plan":
		planID, opts, err := parseExecuteCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExecuteCreativePlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-result":
		planID, opts, err := parseCreativeResultArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeResult(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "validate-creative-plan":
		planID, opts, err := parseValidateCreativePlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ValidateCreativePlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-execute-stub":
		planID, opts, err := parseCreativeExecuteStubArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeExecuteStub(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-creative-outputs":
		planID, opts, err := parseReviewCreativeOutputsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewCreativeOutputs(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-timeline":
		planID, opts, err := parseCreativeTimelineArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeTimeline(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-render-plan":
		planID, opts, err := parseCreativeRenderPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeRenderPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-creative-timeline":
		planID, opts, err := parseReviewCreativeTimelineArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewCreativeTimeline(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-assemble":
		planID, opts, err := parseCreativeAssembleArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeAssemble(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "validate-creative-assemble":
		planID, opts, err := parseValidateCreativeAssembleArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ValidateCreativeAssemble(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-creative-assemble":
		planID, opts, err := parseReviewCreativeAssembleArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewCreativeAssemble(planID, stdout, opts); err != nil {
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
	case "style":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "error: style requires a subcommand: init, inspect, validate")
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "init":
			opts, err := parseStyleInitArgs(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			if err := commands.StyleInit(stdout, opts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		case "inspect":
			opts, err := parseStyleInspectArgs(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			if err := commands.StyleInspect(stdout, opts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		case "validate":
			opts, err := parseStyleValidateArgs(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			if err := commands.StyleValidate(stdout, opts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		default:
			fmt.Fprintf(stderr, "error: unknown style subcommand %q; supported: init, inspect, validate\n", args[1])
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "creative-generate-script":
		planID, opts, err := parseCreativeGenerateScriptArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreativeGenerateScript(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-script":
		planID, opts, err := parseReviewScriptArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewScript(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-caption-variants":
		planID, opts, err := parseCaptionVariantsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CaptionVariants(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-voiceover-text":
		planID, opts, err := parseVoiceoverTextArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VoiceoverTextCommand(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "voiceover-status":
		planID, opts, err := parseVoiceoverStatusArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VoiceoverStatus(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-voiceover":
		planID, opts, err := parseReviewVoiceoverArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewVoiceover(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "validate-voiceover":
		planID, opts, err := parseValidateVoiceoverArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ValidateVoiceover(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "creative-generate-voiceover":
		planID, opts, err := parseGenerateVoiceoverArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.GenerateVoiceover(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-generated-voiceover":
		planID, opts, err := parseReviewGeneratedVoiceoverArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewGeneratedVoiceover(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-caption-variants":
		planID, opts, err := parseReviewCaptionVariantsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewCaptionVariants(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "make":
		inputFile, opts, err := parseMakeArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Make(inputFile, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "makes":
		opts, err := parseMakesArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Makes(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-make":
		makeID, opts, err := parseInspectMakeArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectMake(makeID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "make-result":
		makeID, opts, err := parseMakeResultArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MakeResult(makeID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "revise-make":
		makeID, opts, err := parseReviseMakeArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviseMake(makeID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "make-revisions":
		makeID, opts, err := parseMakeRevisionsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.MakeRevisions(makeID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-make-revision":
		makeID, revisionID, opts, err := parseInspectMakeRevisionArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectMakeRevision(makeID, revisionID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-make-revision":
		makeID, revisionID, opts, err := parseReviewMakeRevisionArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewMakeRevision(makeID, revisionID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-create":
		opts, err := parseJobCreateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobCreate(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "jobs":
		opts, err := parseJobsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Jobs(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-inspect":
		jobID, opts, err := parseJobInspectArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobInspect(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-events":
		jobID, opts, err := parseJobEventsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobEvents(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-approve":
		jobID, opts, err := parseJobApproveArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobApprove(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-reject":
		jobID, opts, err := parseJobRejectArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobReject(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-cancel":
		jobID, opts, err := parseJobCancelArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobCancel(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-run":
		jobID, opts, err := parseJobRunArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobRun(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-result":
		jobID, opts, err := parseJobResultArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobResult(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-validate":
		jobID, opts, err := parseJobValidateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobValidate(jobID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "job-worker":
		opts, err := parseJobWorkerArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.JobWorker(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "daemon":
		subcommand, startOpts, stopOpts, statusOpts, logsOpts, err := parseDaemonArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch subcommand {
		case "start":
			if err := commands.DaemonStart(stdout, startOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		case "stop":
			if err := commands.DaemonStop(stdout, stopOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		case "status":
			if err := commands.DaemonStatus(stdout, statusOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		case "logs":
			if err := commands.DaemonLogs(stdout, logsOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		default:
			fmt.Fprintf(stderr, "error: unknown daemon subcommand %q\n", subcommand)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return 0
	case "queue":
		subcommand, queueOpts, healthOpts, err := parseQueueArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch subcommand {
		case "queue":
			if err := commands.Queue(stdout, queueOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		case "health":
			if err := commands.QueueHealth(stdout, healthOpts); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
		default:
			fmt.Fprintf(stderr, "error: unknown queue subcommand %q\n", subcommand)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return 0
	case "agent-planner-diagnose":
		opts, err := parsePlannerDiagnoseArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPlannerDiagnoseCommand(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-plan":
		opts, err := parseAgentPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPlanCommand(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-plans":
		opts, err := parseAgentPlansArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPlans(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-agent-plan":
		planID, opts, err := parseInspectAgentPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectAgentPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-agent-plan":
		planID, opts, err := parseReviewAgentPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewAgentPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-policy":
		planID, opts, err := parseAgentPolicyArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPolicy(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "approve-agent-plan":
		planID, opts, err := parseApproveAgentPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ApproveAgentPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "reject-agent-plan":
		planID, opts, err := parseRejectAgentPlanArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.RejectAgentPlan(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-plan-to-job":
		planID, opts, err := parseAgentPlanToJobArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPlanToJob(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-plan-jobs":
		planID, opts, err := parseAgentPlanJobsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentPlanJobs(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-run":
		planID, opts, err := parseAgentRunArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentRun(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-graph-run":
		planID, opts, err := parseAgentGraphRunArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentGraphRunCommand(stdout, planID, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "agent-orchestrate":
		opts, err := parseAgentOrchestrateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentOrchestrate(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "visual-requests":
		planID, opts, err := parseVisualRequestsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.VisualRequests(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "execute-visual-requests":
		planID, opts, err := parseExecuteVisualRequestsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ExecuteVisualRequests(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "review-visual-generation":
		planID, opts, err := parseReviewVisualGenerationArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.ReviewVisualGeneration(planID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "create":
		opts, err := parseCreateArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.Create(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "create-result":
		sessionID, opts, err := parseCreateResultArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreateResult(sessionID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "create-sessions":
		opts, err := parseCreateSessionsArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.CreateSessions(stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "inspect-create-session":
		sessionID, opts, err := parseInspectCreateSessionArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.InspectCreateSession(sessionID, stdout, opts); err != nil {
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
	case "agent-result":
		planID, opts, err := parseAgentResultArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.AgentResultCommand(planID, stdout, opts); err != nil {
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
	case "goal-handoff":
		runID, opts, err := parseGoalHandoffArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.GoalHandoffCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "goal-review-bundle":
		runID, opts, err := parseGoalReviewBundleArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.GoalReviewBundleCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "goal-rerank":
		runID, opts, err := parseGoalRerankArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.GoalRerankCommand(runID, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	case "goal-roughcut":
		runID, opts, err := parseGoalRoughcutArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		if err := commands.GoalRoughcutCommand(runID, stdout, opts); err != nil {
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

func parseToolsArgs(args []string) (commands.ToolsOptions, *commands.ToolsValidateOptions, error) {
	toolsOpts := commands.ToolsOptions{}
	validateOpts := &commands.ToolsValidateOptions{}
	validateMode := false
	for _, arg := range args {
		switch arg {
		case "validate":
			validateMode = true
		case "--json":
			toolsOpts.JSON = true
			validateOpts.JSON = true
		case "--strict":
			validateOpts.Strict = true
		case "--check-env":
			validateOpts.CheckEnv = true
		default:
			return toolsOpts, nil, fmt.Errorf("unknown tools flag %q", arg)
		}
	}
	if validateMode {
		return toolsOpts, validateOpts, nil
	}
	if validateOpts.Strict || validateOpts.CheckEnv {
		return toolsOpts, nil, errors.New("--strict and --check-env require tools validate")
	}
	return toolsOpts, nil, nil
}

func parseToolsRequirementsArgs(args []string) (string, commands.ToolsRequirementsOptions, error) {
	opts := commands.ToolsRequirementsOptions{}
	goal := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--goal":
			if index+1 >= len(args) {
				return "", opts, errors.New("--goal requires a value")
			}
			index++
			goal = args[index]
		case "--json":
			opts.JSON = true
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				goal = value
				continue
			}
			return "", opts, fmt.Errorf("unknown tools requirements flag %q", arg)
		}
	}
	if strings.TrimSpace(goal) == "" {
		return "", opts, errors.New("tools requirements requires --goal")
	}
	return goal, opts, nil
}

func parseCreativePlanArgs(args []string) (string, commands.CreativePlanOptions, error) {
	opts := commands.CreativePlanOptions{WriteArtifact: true}
	inputPath := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--goal":
			if index+1 >= len(args) {
				return "", opts, errors.New("--goal requires a value")
			}
			index++
			opts.Goal = args[index]
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				opts.Goal = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-plan flag %q", arg)
			}
			if inputPath != "" {
				return "", opts, errors.New("creative-plan requires exactly one input file")
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", opts, errors.New("creative-plan requires exactly one input file")
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return "", opts, errors.New("creative-plan requires --goal")
	}
	return inputPath, opts, nil
}

func parseInspectCreativePlanArgs(args []string) (string, commands.InspectCreativePlanOptions, error) {
	opts := commands.InspectCreativePlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown inspect-creative-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("inspect-creative-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("inspect-creative-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewCreativePlanArgs(args []string) (string, commands.ReviewCreativePlanOptions, error) {
	opts := commands.ReviewCreativePlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-creative-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-creative-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-creative-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseApproveCreativePlanArgs(args []string) (string, error) {
	planID := ""
	for _, arg := range args {
		if len(arg) > 0 && arg[0] == '-' {
			return "", fmt.Errorf("approve-creative-plan does not accept flags; got %q", arg)
		}
		if planID != "" {
			return "", errors.New("approve-creative-plan requires exactly one plan id")
		}
		planID = arg
	}
	if planID == "" {
		return "", errors.New("approve-creative-plan requires exactly one plan id")
	}
	return planID, nil
}

func parseCreativePlanEventsArgs(args []string) (string, commands.CreativePlanEventsOptions, error) {
	opts := commands.CreativePlanEventsOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-plan-events flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-plan-events requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-plan-events requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativePreviewArgs(args []string) (string, commands.CreativePreviewOptions, error) {
	opts := commands.CreativePreviewOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		case "--overwrite":
			opts.Overwrite = true
		case "--check-env":
			opts.CheckEnv = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-preview flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-preview requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-preview requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseExecuteCreativePlanArgs(args []string) (string, commands.ExecuteCreativePlanOptions, error) {
	opts := commands.ExecuteCreativePlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--dry-run":
			opts.DryRun = true
		case "--strict":
			opts.Strict = true
		case "--check-env":
			opts.CheckEnv = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown execute-creative-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("execute-creative-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("execute-creative-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativeResultArgs(args []string) (string, commands.CreativeResultOptions, error) {
	opts := commands.CreativeResultOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-result flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-result requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-result requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseValidateCreativePlanArgs(args []string) (string, commands.ValidateCreativePlanOptions, error) {
	opts := commands.ValidateCreativePlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown validate-creative-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("validate-creative-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("validate-creative-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativeExecuteStubArgs(args []string) (string, commands.CreativeExecuteStubOptions, error) {
	opts := commands.CreativeExecuteStubOptions{}
	planID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--dry-run":
			opts.DryRun = true
		case "--step-type":
			if index+1 >= len(args) {
				return "", opts, errors.New("--step-type requires a value")
			}
			index++
			opts.StepType = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--step-type="); ok {
				opts.StepType = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-execute-stub flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-execute-stub requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-execute-stub requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewCreativeOutputsArgs(args []string) (string, commands.ReviewCreativeOutputsOptions, error) {
	opts := commands.ReviewCreativeOutputsOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-creative-outputs flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-creative-outputs requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-creative-outputs requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativeTimelineArgs(args []string) (string, commands.CreativeTimelineOptions, error) {
	opts := commands.CreativeTimelineOptions{}
	planID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--overwrite":
			opts.Overwrite = true
		case arg == "--prefer-goal":
			opts.PreferGoal = true
		case arg == "--run-id":
			if i+1 >= len(args) {
				return "", opts, errors.New("--run-id requires a value")
			}
			i++
			opts.RunID = args[i]
		case strings.HasPrefix(arg, "--run-id="):
			opts.RunID = strings.TrimPrefix(arg, "--run-id=")
		case len(arg) > 0 && arg[0] == '-':
			return "", opts, fmt.Errorf("unknown creative-timeline flag %q", arg)
		default:
			if planID != "" {
				return "", opts, errors.New("creative-timeline requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-timeline requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativeRenderPlanArgs(args []string) (string, commands.CreativeRenderPlanOptions, error) {
	opts := commands.CreativeRenderPlanOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--overwrite":
			opts.Overwrite = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown creative-render-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-render-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-render-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewCreativeTimelineArgs(args []string) (string, commands.ReviewCreativeTimelineOptions, error) {
	opts := commands.ReviewCreativeTimelineOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-creative-timeline flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-creative-timeline requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-creative-timeline requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseCreativeAssembleArgs(args []string) (string, commands.CreativeAssembleOptions, error) {
	opts := commands.CreativeAssembleOptions{}
	planID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--overwrite":
			opts.Overwrite = true
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--keep-work":
			opts.KeepWork = true
		case arg == "--mode":
			if i+1 >= len(args) {
				return "", opts, errors.New("--mode requires a value")
			}
			i++
			opts.Mode = args[i]
		case strings.HasPrefix(arg, "--mode="):
			opts.Mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--max-clips":
			if i+1 >= len(args) {
				return "", opts, errors.New("--max-clips requires a value")
			}
			i++
			n, err := parseIntFlag("--max-clips", args[i])
			if err != nil {
				return "", opts, err
			}
			opts.MaxClips = n
		case strings.HasPrefix(arg, "--max-clips="):
			n, err := parseIntFlag("--max-clips", strings.TrimPrefix(arg, "--max-clips="))
			if err != nil {
				return "", opts, err
			}
			opts.MaxClips = n
		case arg == "--burn-captions":
			opts.BurnCaptions = true
		case arg == "--allow-missing-captions":
			opts.AllowMissingCaptions = true
		case arg == "--captions":
			if i+1 >= len(args) {
				return "", opts, errors.New("--captions requires a value")
			}
			i++
			opts.CaptionsPath = args[i]
		case strings.HasPrefix(arg, "--captions="):
			opts.CaptionsPath = strings.TrimPrefix(arg, "--captions=")
		case arg == "--mix-voiceover":
			opts.MixVoiceover = true
		case arg == "--allow-missing-voiceover":
			opts.AllowMissingVoiceover = true
		case arg == "--voiceover":
			if i+1 >= len(args) {
				return "", opts, errors.New("--voiceover requires a value")
			}
			i++
			opts.VoiceoverPath = args[i]
		case strings.HasPrefix(arg, "--voiceover="):
			opts.VoiceoverPath = strings.TrimPrefix(arg, "--voiceover=")
		case arg == "--run-id":
			if i+1 >= len(args) {
				return "", opts, errors.New("--run-id requires a value")
			}
			i++
			opts.RunID = args[i]
		case strings.HasPrefix(arg, "--run-id="):
			opts.RunID = strings.TrimPrefix(arg, "--run-id=")
		case arg == "--platform":
			if i+1 >= len(args) {
				return "", opts, errors.New("--platform requires a value")
			}
			i++
			opts.Platform = args[i]
		case strings.HasPrefix(arg, "--platform="):
			opts.Platform = strings.TrimPrefix(arg, "--platform=")
		case arg == "--fit":
			if i+1 >= len(args) {
				return "", opts, errors.New("--fit requires a value")
			}
			i++
			opts.Fit = args[i]
		case strings.HasPrefix(arg, "--fit="):
			opts.Fit = strings.TrimPrefix(arg, "--fit=")
		case arg == "--background":
			if i+1 >= len(args) {
				return "", opts, errors.New("--background requires a value")
			}
			i++
			opts.Background = args[i]
		case strings.HasPrefix(arg, "--background="):
			opts.Background = strings.TrimPrefix(arg, "--background=")
		case arg == "--caption-position":
			if i+1 >= len(args) {
				return "", opts, errors.New("--caption-position requires a value")
			}
			i++
			opts.CaptionPosition = args[i]
		case strings.HasPrefix(arg, "--caption-position="):
			opts.CaptionPosition = strings.TrimPrefix(arg, "--caption-position=")
		case arg == "--caption-margin":
			if i+1 >= len(args) {
				return "", opts, errors.New("--caption-margin requires a value")
			}
			i++
			n, err := parseIntFlag("--caption-margin", args[i])
			if err != nil {
				return "", opts, err
			}
			opts.CaptionMargin = n
		case strings.HasPrefix(arg, "--caption-margin="):
			n, err := parseIntFlag("--caption-margin", strings.TrimPrefix(arg, "--caption-margin="))
			if err != nil {
				return "", opts, err
			}
			opts.CaptionMargin = n
		case arg == "--caption-style":
			if i+1 >= len(args) {
				return "", opts, errors.New("--caption-style requires a value")
			}
			i++
			opts.CaptionStyle = args[i]
		case strings.HasPrefix(arg, "--caption-style="):
			opts.CaptionStyle = strings.TrimPrefix(arg, "--caption-style=")
		case len(arg) > 0 && arg[0] == '-':
			return "", opts, fmt.Errorf("unknown creative-assemble flag %q", arg)
		default:
			if planID != "" {
				return "", opts, errors.New("creative-assemble requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-assemble requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseValidateCreativeAssembleArgs(args []string) (string, commands.ValidateCreativeAssembleOptions, error) {
	opts := commands.ValidateCreativeAssembleOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown validate-creative-assemble flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("validate-creative-assemble requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("validate-creative-assemble requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewCreativeAssembleArgs(args []string) (string, commands.ReviewCreativeAssembleOptions, error) {
	opts := commands.ReviewCreativeAssembleOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown review-creative-assemble flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-creative-assemble requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-creative-assemble requires exactly one plan id")
	}
	return planID, opts, nil
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
		case "--prefer-goal-roughcut":
			opts.PreferGoalRoughcut = true
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
		case "--prefer-goal-roughcut":
			opts.PreferGoalRoughcut = true
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

func parseGoalHandoffArgs(args []string) (string, commands.GoalHandoffOptions, error) {
	opts := commands.GoalHandoffOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown goal-handoff flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("goal-handoff requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("goal-handoff requires exactly one run id")
	}
	return runID, opts, nil
}

func parseGoalReviewBundleArgs(args []string) (string, commands.GoalReviewBundleOptions, error) {
	var runID string
	opts := commands.GoalReviewBundleOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--overwrite":
			opts.Overwrite = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown goal-review-bundle flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("goal-review-bundle requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("goal-review-bundle requires exactly one run id")
	}
	return runID, opts, nil
}

func parseGoalRerankArgs(args []string) (string, commands.GoalRerankOptions, error) {
	opts := commands.GoalRerankOptions{}
	runID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--use-ollama":
			opts.UseOllama = true
		case "--fallback-deterministic":
			opts.FallbackDeterministic = true
		case "--goal":
			if index+1 >= len(args) {
				return "", opts, errors.New("--goal requires a value")
			}
			index++
			opts.Goal = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				opts.Goal = value
				continue
			}
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown goal-rerank flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("goal-rerank requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("goal-rerank requires exactly one run id")
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return "", opts, errors.New("--goal is required")
	}
	return runID, opts, nil
}

func parseGoalRoughcutArgs(args []string) (string, commands.GoalRoughcutOptions, error) {
	opts := commands.GoalRoughcutOptions{}
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return "", opts, fmt.Errorf("unknown goal-roughcut flag %q", arg)
			}
			if runID != "" {
				return "", opts, errors.New("goal-roughcut requires exactly one run id")
			}
			runID = arg
		}
	}
	if runID == "" {
		return "", opts, errors.New("goal-roughcut requires exactly one run id")
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

func parseMakeArgs(args []string) (string, commands.MakeOptions, error) {
	opts := commands.MakeOptions{VoiceoverStability: -1, VoiceoverSimilarityBoost: -1}
	inputFile := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
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
		case "--skip-pipeline":
			if index+1 >= len(args) {
				return "", opts, errors.New("--skip-pipeline requires a run_id value")
			}
			index++
			opts.SkipPipeline = args[index]
		case "--strict-input":
			opts.StrictInput = true
		case "--export":
			opts.Export = true
		case "--require-export":
			opts.RequireExport = true
		case "--yes":
			opts.Yes = true
		case "--dry-run":
			opts.DryRun = true
		case "--burn-captions":
			opts.BurnCaptions = true
		case "--allow-missing-captions":
			opts.AllowMissingCaptions = true
		case "--mix-voiceover":
			opts.MixVoiceover = true
		case "--voiceover":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover requires a value")
			}
			index++
			opts.VoiceoverPath = args[index]
		case "--allow-missing-voiceover":
			opts.AllowMissingVoiceover = true
		case "--mode":
			if index+1 >= len(args) {
				return "", opts, errors.New("--mode requires a value")
			}
			index++
			opts.Mode = args[index]
		case "--goal-aware":
			opts.GoalAware = true
		case "--use-ollama-goal":
			opts.UseOllamaGoal = true
		case "--generate-script":
			opts.GenerateScript = true
		case "--script-fallback-stub":
			opts.ScriptFallbackStub = true
		case "--no-style":
			opts.ScriptNoStyle = true
		case "--style-dir":
			if index+1 >= len(args) {
				return "", opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.ScriptStyleDir = args[index]
		case "--script-route":
			if index+1 >= len(args) {
				return "", opts, errors.New("--script-route requires a value")
			}
			index++
			opts.ScriptRoute = args[index]
		case "--script-model":
			if index+1 >= len(args) {
				return "", opts, errors.New("--script-model requires a value")
			}
			index++
			opts.ScriptModelEntry = args[index]
		case "--script-max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--script-max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--script-max-words must be an integer")
			}
			opts.ScriptMaxWords = n
		case "--script-tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--script-tone requires a value")
			}
			index++
			opts.ScriptTone = args[index]
		case "--prepare-voiceover":
			opts.PrepareVoiceover = true
		case "--require-voiceover":
			opts.RequireVoiceover = true
		case "--voiceover-max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--voiceover-max-words must be an integer")
			}
			opts.VoiceoverMaxWords = n
		case "--voiceover-tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-tone requires a value")
			}
			index++
			opts.VoiceoverTone = args[index]
		case "--generate-voiceover":
			opts.GenerateVoiceover = true
		case "--voiceover-route":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-route requires a value")
			}
			index++
			opts.VoiceoverRoute = args[index]
		case "--voiceover-backend":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-backend requires a value")
			}
			index++
			opts.VoiceoverBackend = args[index]
		case "--voiceover-voice-id":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-voice-id requires a value")
			}
			index++
			opts.VoiceoverVoiceID = args[index]
		case "--voiceover-timeout-seconds":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-timeout-seconds requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--voiceover-timeout-seconds must be an integer")
			}
			opts.VoiceoverTimeoutSeconds = n
		case "--voiceover-dry-run":
			opts.VoiceoverDryRun = true
		case "--voiceover-check-env":
			opts.VoiceoverCheckEnv = true
		case "--allow-missing-generated-voiceover":
			opts.AllowMissingGeneratedVoiceover = true
		case "--platform":
			if index+1 >= len(args) {
				return "", opts, errors.New("--platform requires a value")
			}
			index++
			opts.Platform = args[index]
		case "--fit":
			if index+1 >= len(args) {
				return "", opts, errors.New("--fit requires a value")
			}
			index++
			opts.Fit = args[index]
		case "--background":
			if index+1 >= len(args) {
				return "", opts, errors.New("--background requires a value")
			}
			index++
			opts.Background = args[index]
		case "--generate-captions":
			opts.GenerateCaptions = true
		case "--caption-fallback-stub":
			opts.CaptionFallbackStub = true
		case "--caption-count":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-count requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--caption-count must be an integer")
			}
			opts.CaptionCount = n
		case "--caption-max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--caption-max-words must be an integer")
			}
			opts.CaptionMaxWords = n
		case "--caption-tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-tone requires a value")
			}
			index++
			opts.CaptionTone = args[index]
		case "--caption-position":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-position requires a value")
			}
			index++
			opts.CaptionPosition = args[index]
		case "--caption-margin":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-margin requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--caption-margin must be an integer")
			}
			opts.CaptionMargin = n
		case "--caption-style":
			if index+1 >= len(args) {
				return "", opts, errors.New("--caption-style requires a value")
			}
			index++
			opts.CaptionStyle = args[index]
		case "--voiceover-stability":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-stability requires a value")
			}
			index++
			v, err := strconv.ParseFloat(args[index], 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--voiceover-stability must be a float between 0.0 and 1.0")
			}
			opts.VoiceoverStability = v
		case "--voiceover-similarity-boost":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-similarity-boost requires a value")
			}
			index++
			v, err := strconv.ParseFloat(args[index], 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--voiceover-similarity-boost must be a float between 0.0 and 1.0")
			}
			opts.VoiceoverSimilarityBoost = v
		case "--voiceover-output-format":
			if index+1 >= len(args) {
				return "", opts, errors.New("--voiceover-output-format requires a value")
			}
			index++
			opts.VoiceoverOutputFormat = args[index]
		case "--keep-work":
			opts.KeepWork = true
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				opts.Goal = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--preset="); ok {
				opts.Preset = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--skip-pipeline="); ok {
				opts.SkipPipeline = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--mode="); ok {
				opts.Mode = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--voiceover="); ok {
				opts.VoiceoverPath = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.ScriptStyleDir = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--script-route="); ok {
				opts.ScriptRoute = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--script-model="); ok {
				opts.ScriptModelEntry = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--script-tone="); ok {
				opts.ScriptTone = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for make", arg)
			}
			if inputFile != "" {
				return "", opts, errors.New("make: unexpected extra argument")
			}
			inputFile = arg
		}
	}
	// input file is optional when --skip-pipeline is set
	if inputFile == "" && opts.SkipPipeline == "" && !opts.DryRun {
		return "", opts, errors.New("make requires an input file argument (or --skip-pipeline <run_id>)")
	}
	// inherit python interpreter from env/config
	if opts.PythonInterpreter == "" {
		if p := os.Getenv("BYOM_VIDEO_PYTHON"); p != "" {
			opts.PythonInterpreter = p
		}
	}
	return inputFile, opts, nil
}

func parseMakeResultArgs(args []string) (string, commands.MakeResultOptions, error) {
	opts := commands.MakeResultOptions{}
	makeID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for make-result", arg)
			}
			if makeID != "" {
				return "", opts, errors.New("make-result: unexpected extra argument")
			}
			makeID = arg
		}
	}
	if makeID == "" {
		return "", opts, errors.New("make-result requires a make_id argument")
	}
	return makeID, opts, nil
}

func parseMakesArgs(args []string) (commands.MakesOptions, error) {
	opts := commands.MakesOptions{}
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown flag %q for makes", arg)
		}
	}
	return opts, nil
}

func parseInspectMakeArgs(args []string) (string, commands.InspectMakeOptions, error) {
	opts := commands.InspectMakeOptions{}
	makeID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for inspect-make", arg)
			}
			if makeID != "" {
				return "", opts, errors.New("inspect-make: unexpected extra argument")
			}
			makeID = arg
		}
	}
	if makeID == "" {
		return "", opts, errors.New("inspect-make requires a make_id argument")
	}
	return makeID, opts, nil
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
		case "--goal-aware":
			opts.GoalAware = true
		case "--goal-use-ollama":
			opts.GoalUseOllama = true
		case "--goal-fallback-deterministic":
			opts.GoalFallbackDeterministic = true
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
	if !opts.GoalAware && (opts.GoalUseOllama || opts.GoalFallbackDeterministic) {
		return "", opts, errors.New("--goal-use-ollama and --goal-fallback-deterministic require --goal-aware")
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

func parseAgentResultArgs(args []string) (string, commands.AgentResultOptions, error) {
	var planID string
	opts := commands.AgentResultOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-result flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("agent-result requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("agent-result requires exactly one plan id")
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

func parseDoctorArgs(args []string) (commands.DoctorOptions, error) {
	var opts commands.DoctorOptions
	for _, a := range args {
		switch a {
		case "--transcription":
			opts.Transcription = true
		case "--media":
			opts.Media = true
		default:
			return opts, fmt.Errorf("unknown flag %q for doctor", a)
		}
	}
	return opts, nil
}

// ---- style parse functions ----

func parseStyleInitArgs(args []string) (commands.StyleInitOptions, error) {
	opts := commands.StyleInitOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--force":
			opts.Force = true
		case "--json":
			opts.JSON = true
		case "--style-dir":
			if index+1 >= len(args) {
				return opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.StyleDir = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.StyleDir = value
				continue
			}
			return opts, fmt.Errorf("unknown flag %q for style init", arg)
		}
	}
	return opts, nil
}

func parseStyleInspectArgs(args []string) (commands.StyleInspectOptions, error) {
	opts := commands.StyleInspectOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--style-dir":
			if index+1 >= len(args) {
				return opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.StyleDir = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.StyleDir = value
				continue
			}
			return opts, fmt.Errorf("unknown flag %q for style inspect", arg)
		}
	}
	return opts, nil
}

func parseStyleValidateArgs(args []string) (commands.StyleValidateOptions, error) {
	opts := commands.StyleValidateOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		case "--style-dir":
			if index+1 >= len(args) {
				return opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.StyleDir = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.StyleDir = value
				continue
			}
			return opts, fmt.Errorf("unknown flag %q for style validate", arg)
		}
	}
	return opts, nil
}

// ---- creative-generate-script parse functions ----

func parseCreativeGenerateScriptArgs(args []string) (string, commands.CreativeGenerateScriptOptions, error) {
	opts := commands.CreativeGenerateScriptOptions{}
	planID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--no-style":
			opts.NoStyle = true
		case "--fallback-stub":
			opts.FallbackStub = true
		case "--style-dir":
			if index+1 >= len(args) {
				return "", opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.StyleDir = args[index]
		case "--route":
			if index+1 >= len(args) {
				return "", opts, errors.New("--route requires a value")
			}
			index++
			opts.Route = args[index]
		case "--model":
			if index+1 >= len(args) {
				return "", opts, errors.New("--model requires a value")
			}
			index++
			opts.ModelEntry = args[index]
		case "--max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--max-words must be an integer")
			}
			opts.MaxWords = n
		case "--tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--tone requires a value")
			}
			index++
			opts.Tone = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.StyleDir = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--route="); ok {
				opts.Route = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--model="); ok {
				opts.ModelEntry = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--tone="); ok {
				opts.Tone = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for creative-generate-script", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-generate-script: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-generate-script requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- review-script parse functions ----

func parseReviewScriptArgs(args []string) (string, commands.ReviewScriptOptions, error) {
	opts := commands.ReviewScriptOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for review-script", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-script: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-script requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- creative-caption-variants parse functions ----

func parseCaptionVariantsArgs(args []string) (string, commands.CaptionVariantsOptions, error) {
	opts := commands.CaptionVariantsOptions{}
	planID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--no-style":
			opts.NoStyle = true
		case "--fallback-stub":
			opts.FallbackStub = true
		case "--style-dir":
			if index+1 >= len(args) {
				return "", opts, errors.New("--style-dir requires a value")
			}
			index++
			opts.StyleDir = args[index]
		case "--route":
			if index+1 >= len(args) {
				return "", opts, errors.New("--route requires a value")
			}
			index++
			opts.Route = args[index]
		case "--model":
			if index+1 >= len(args) {
				return "", opts, errors.New("--model requires a value")
			}
			index++
			opts.ModelEntry = args[index]
		case "--count":
			if index+1 >= len(args) {
				return "", opts, errors.New("--count requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--count must be an integer")
			}
			opts.Count = n
		case "--max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--max-words must be an integer")
			}
			opts.MaxWords = n
		case "--tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--tone requires a value")
			}
			index++
			opts.Tone = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--style-dir="); ok {
				opts.StyleDir = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--route="); ok {
				opts.Route = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--model="); ok {
				opts.ModelEntry = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--tone="); ok {
				opts.Tone = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for creative-caption-variants", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-caption-variants: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-caption-variants requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

func parseReviewCaptionVariantsArgs(args []string) (string, commands.ReviewCaptionVariantsOptions, error) {
	opts := commands.ReviewCaptionVariantsOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for review-caption-variants", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-caption-variants: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-caption-variants requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- creative-voiceover-text parse ----

func parseVoiceoverTextArgs(args []string) (string, commands.VoiceoverTextOptions, error) {
	opts := commands.VoiceoverTextOptions{}
	planID := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--overwrite":
			opts.Overwrite = true
		case "--json":
			opts.JSON = true
		case "--max-words":
			if index+1 >= len(args) {
				return "", opts, errors.New("--max-words requires a value")
			}
			index++
			n, err := strconv.Atoi(args[index])
			if err != nil {
				return "", opts, fmt.Errorf("--max-words must be an integer")
			}
			opts.MaxWords = n
		case "--tone":
			if index+1 >= len(args) {
				return "", opts, errors.New("--tone requires a value")
			}
			index++
			opts.Tone = args[index]
		case "--source":
			if index+1 >= len(args) {
				return "", opts, errors.New("--source requires a value")
			}
			index++
			opts.Source = args[index]
		default:
			if value, ok := strings.CutPrefix(arg, "--tone="); ok {
				opts.Tone = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--source="); ok {
				opts.Source = value
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for creative-voiceover-text", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-voiceover-text: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-voiceover-text requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- voiceover-status parse ----

func parseVoiceoverStatusArgs(args []string) (string, commands.VoiceoverStatusOptions, error) {
	opts := commands.VoiceoverStatusOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for voiceover-status", arg)
			}
			if planID != "" {
				return "", opts, errors.New("voiceover-status: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("voiceover-status requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- review-voiceover parse ----

func parseReviewVoiceoverArgs(args []string) (string, commands.ReviewVoiceoverOptions, error) {
	opts := commands.ReviewVoiceoverOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for review-voiceover", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-voiceover: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-voiceover requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

// ---- validate-voiceover parse ----

func parseValidateVoiceoverArgs(args []string) (string, commands.ValidateVoiceoverOptions, error) {
	opts := commands.ValidateVoiceoverOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--require-audio":
			opts.RequireAudio = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for validate-voiceover", arg)
			}
			if planID != "" {
				return "", opts, errors.New("validate-voiceover: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("validate-voiceover requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

func parseGenerateVoiceoverArgs(args []string) (string, commands.GenerateVoiceoverOptions, error) {
	opts := commands.GenerateVoiceoverOptions{Stability: -1, SimilarityBoost: -1}
	planID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--overwrite":
			opts.Overwrite = true
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--check-env":
			opts.CheckEnv = true
		case arg == "--prepare-text":
			opts.PrepareText = true
		case arg == "--route":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--route requires a value")
			}
			opts.Route = args[i]
		case strings.HasPrefix(arg, "--route="):
			opts.Route = strings.TrimPrefix(arg, "--route=")
		case arg == "--backend":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--backend requires a value")
			}
			opts.Backend = args[i]
		case strings.HasPrefix(arg, "--backend="):
			opts.Backend = strings.TrimPrefix(arg, "--backend=")
		case arg == "--voice-id":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--voice-id requires a value")
			}
			opts.VoiceID = args[i]
		case strings.HasPrefix(arg, "--voice-id="):
			opts.VoiceID = strings.TrimPrefix(arg, "--voice-id=")
		case arg == "--model":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--model requires a value")
			}
			opts.Model = args[i]
		case strings.HasPrefix(arg, "--model="):
			opts.Model = strings.TrimPrefix(arg, "--model=")
		case arg == "--timeout-seconds":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--timeout-seconds requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return "", opts, fmt.Errorf("--timeout-seconds must be an integer")
			}
			opts.TimeoutSeconds = n
		case arg == "--text":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--text requires a value")
			}
			opts.Text = args[i]
		case strings.HasPrefix(arg, "--text="):
			opts.Text = strings.TrimPrefix(arg, "--text=")
		case arg == "--stability":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--stability requires a value")
			}
			v, err := strconv.ParseFloat(args[i], 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--stability must be a float between 0.0 and 1.0")
			}
			opts.Stability = v
		case strings.HasPrefix(arg, "--stability="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--stability="), 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--stability must be a float between 0.0 and 1.0")
			}
			opts.Stability = v
		case arg == "--similarity-boost":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--similarity-boost requires a value")
			}
			v, err := strconv.ParseFloat(args[i], 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--similarity-boost must be a float between 0.0 and 1.0")
			}
			opts.SimilarityBoost = v
		case strings.HasPrefix(arg, "--similarity-boost="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--similarity-boost="), 64)
			if err != nil || v < 0 || v > 1 {
				return "", opts, fmt.Errorf("--similarity-boost must be a float between 0.0 and 1.0")
			}
			opts.SimilarityBoost = v
		case arg == "--output-format":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--output-format requires a value")
			}
			opts.OutputFormat = args[i]
		case strings.HasPrefix(arg, "--output-format="):
			opts.OutputFormat = strings.TrimPrefix(arg, "--output-format=")
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for creative-generate-voiceover", arg)
			}
			if planID != "" {
				return "", opts, errors.New("creative-generate-voiceover: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("creative-generate-voiceover requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

func parseReviewGeneratedVoiceoverArgs(args []string) (string, commands.ReviewGeneratedVoiceoverOptions, error) {
	opts := commands.ReviewGeneratedVoiceoverOptions{}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for review-generated-voiceover", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-generated-voiceover: unexpected extra argument")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-generated-voiceover requires a creative_plan_id argument")
	}
	return planID, opts, nil
}

func parseReviseMakeArgs(args []string) (string, commands.ReviseMakeOptions, error) {
	opts := commands.ReviseMakeOptions{}
	makeID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--request":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--request requires a value")
			}
			opts.Request = args[i]
		case strings.HasPrefix(arg, "--request="):
			opts.Request = strings.TrimPrefix(arg, "--request=")
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--json":
			opts.JSON = true
		case arg == "--yes":
			opts.Yes = true
		case arg == "--overwrite":
			opts.Overwrite = true
		case arg == "--new-make":
			opts.NewMake = true
		case arg == "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case arg == "--fallback-stub":
			opts.FallbackStub = true
		case arg == "--reassemble":
			opts.Reassemble = true
		case arg == "--validate":
			opts.Validate = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for revise-make", arg)
			}
			if makeID != "" {
				return "", opts, errors.New("revise-make: unexpected extra argument")
			}
			makeID = arg
		}
	}
	if makeID == "" {
		return "", opts, errors.New("revise-make requires a make_id argument")
	}
	return makeID, opts, nil
}

func parseMakeRevisionsArgs(args []string) (string, commands.MakeRevisionsOptions, error) {
	opts := commands.MakeRevisionsOptions{}
	makeID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for make-revisions", arg)
			}
			if makeID != "" {
				return "", opts, errors.New("make-revisions: unexpected extra argument")
			}
			makeID = arg
		}
	}
	if makeID == "" {
		return "", opts, errors.New("make-revisions requires a make_id argument")
	}
	return makeID, opts, nil
}

func parseInspectMakeRevisionArgs(args []string) (string, string, commands.InspectMakeRevisionOptions, error) {
	opts := commands.InspectMakeRevisionOptions{}
	makeID := ""
	revisionID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", opts, fmt.Errorf("unknown flag %q for inspect-make-revision", arg)
			}
			if makeID == "" {
				makeID = arg
			} else if revisionID == "" {
				revisionID = arg
			} else {
				return "", "", opts, errors.New("inspect-make-revision: unexpected extra argument")
			}
		}
	}
	if makeID == "" {
		return "", "", opts, errors.New("inspect-make-revision requires a make_id argument")
	}
	if revisionID == "" {
		return "", "", opts, errors.New("inspect-make-revision requires a revision_id argument")
	}
	return makeID, revisionID, opts, nil
}

func parseReviewMakeRevisionArgs(args []string) (string, string, commands.ReviewMakeRevisionOptions, error) {
	opts := commands.ReviewMakeRevisionOptions{}
	makeID := ""
	revisionID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", "", opts, fmt.Errorf("unknown flag %q for review-make-revision", arg)
			}
			if makeID == "" {
				makeID = arg
			} else if revisionID == "" {
				revisionID = arg
			} else {
				return "", "", opts, errors.New("review-make-revision: unexpected extra argument")
			}
		}
	}
	if makeID == "" {
		return "", "", opts, errors.New("review-make-revision requires a make_id argument")
	}
	if revisionID == "" {
		return "", "", opts, errors.New("review-make-revision requires a revision_id argument")
	}
	return makeID, revisionID, opts, nil
}

func parseJobCreateArgs(args []string) (commands.JobCreateOptions, error) {
	opts := commands.JobCreateOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--type":
			i++
			if i >= len(args) {
				return opts, errors.New("--type requires a value")
			}
			opts.ActionType = args[i]
		case "--goal":
			i++
			if i >= len(args) {
				return opts, errors.New("--goal requires a value")
			}
			opts.Goal = args[i]
		case "--preset":
			i++
			if i >= len(args) {
				return opts, errors.New("--preset requires a value")
			}
			opts.Preset = args[i]
		case "--make-id":
			i++
			if i >= len(args) {
				return opts, errors.New("--make-id requires a value")
			}
			opts.MakeID = args[i]
		case "--request":
			i++
			if i >= len(args) {
				return opts, errors.New("--request requires a value")
			}
			opts.Request = args[i]
		case "--reassemble":
			opts.Reassemble = true
		case "--plan-id":
			i++
			if i >= len(args) {
				return opts, errors.New("--plan-id requires a value")
			}
			opts.PlanID = args[i]
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--allow-external-network":
			opts.AllowExternalNetwork = true
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag %q for job-create", arg)
			}
			return opts, fmt.Errorf("job-create: unexpected positional argument %q; use --type to specify action type", arg)
		}
	}
	return opts, nil
}

func parseJobsArgs(args []string) (commands.JobsOptions, error) {
	opts := commands.JobsOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--filter":
			i++
			if i >= len(args) {
				return opts, errors.New("--filter requires a value")
			}
			opts.Filter = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, fmt.Errorf("--limit: invalid number %q", args[i])
			}
			opts.Limit = n
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag %q for jobs", arg)
			}
			return opts, fmt.Errorf("jobs: unexpected argument %q", arg)
		}
	}
	return opts, nil
}

func parseJobInspectArgs(args []string) (string, commands.JobInspectOptions, error) {
	opts := commands.JobInspectOptions{}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-inspect", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-inspect: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-inspect requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobEventsArgs(args []string) (string, commands.JobEventsOptions, error) {
	opts := commands.JobEventsOptions{}
	jobID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--limit":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return "", opts, fmt.Errorf("--limit: invalid number %q", args[i])
			}
			opts.Limit = n
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-events", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-events: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-events requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobApproveArgs(args []string) (string, commands.JobApproveOptions, error) {
	opts := commands.JobApproveOptions{}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-approve", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-approve: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-approve requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobRejectArgs(args []string) (string, commands.JobRejectOptions, error) {
	opts := commands.JobRejectOptions{}
	jobID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--reason":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--reason requires a value")
			}
			opts.Reason = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-reject", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-reject: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-reject requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobCancelArgs(args []string) (string, commands.JobCancelOptions, error) {
	opts := commands.JobCancelOptions{}
	jobID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--reason":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--reason requires a value")
			}
			opts.Reason = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-cancel", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-cancel: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-cancel requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobRunArgs(args []string) (string, commands.JobRunOptions, error) {
	opts := commands.JobRunOptions{}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-run", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-run: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-run requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobResultArgs(args []string) (string, commands.JobResultOptions, error) {
	opts := commands.JobResultOptions{}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-result", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-result: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-result requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobValidateArgs(args []string) (string, commands.JobValidateOptions, error) {
	opts := commands.JobValidateOptions{}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", opts, fmt.Errorf("unknown flag %q for job-validate", arg)
			}
			if jobID != "" {
				return "", opts, errors.New("job-validate: unexpected extra argument")
			}
			jobID = arg
		}
	}
	if jobID == "" {
		return "", opts, errors.New("job-validate requires a job_id argument")
	}
	return jobID, opts, nil
}

func parseJobWorkerArgs(args []string) (commands.JobWorkerOptions, error) {
	opts := commands.JobWorkerOptions{Interval: 10 * time.Second}
	maxJobsSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--once":
			opts.Once = true
		case "--loop":
			opts.Loop = true
		case "--status":
			opts.Status = true
		case "--json":
			opts.JSON = true
		case "--dry-run":
			opts.DryRun = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--fail-fast":
			opts.FailFast = true
		case "--force-lock":
			opts.ForceLock = true
		case "--interval":
			i++
			if i >= len(args) {
				return opts, errors.New("--interval requires a value")
			}
			duration, err := time.ParseDuration(args[i])
			if err != nil {
				return opts, fmt.Errorf("--interval: invalid duration %q", args[i])
			}
			opts.Interval = duration
		case "--max-jobs":
			i++
			if i >= len(args) {
				return opts, errors.New("--max-jobs requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, fmt.Errorf("--max-jobs: invalid number %q", args[i])
			}
			opts.MaxJobs = n
			maxJobsSet = true
		default:
			if value, ok := strings.CutPrefix(arg, "--interval="); ok {
				duration, err := time.ParseDuration(value)
				if err != nil {
					return opts, fmt.Errorf("--interval: invalid duration %q", value)
				}
				opts.Interval = duration
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--max-jobs="); ok {
				n, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("--max-jobs: invalid number %q", value)
				}
				opts.MaxJobs = n
				maxJobsSet = true
				continue
			}
			return opts, fmt.Errorf("unknown flag %q for job-worker", arg)
		}
	}
	if !opts.Once && !opts.Loop && !opts.Status {
		return opts, errors.New("job-worker requires --once, --loop, or --status")
	}
	if opts.Status && (opts.Once || opts.Loop) {
		return opts, errors.New("--status cannot be combined with --once or --loop")
	}
	if opts.Interval <= 0 {
		return opts, errors.New("--interval must be positive")
	}
	if maxJobsSet && opts.MaxJobs < 0 {
		return opts, errors.New("--max-jobs must be zero or positive")
	}
	return opts, nil
}

func parseDaemonArgs(args []string) (string, commands.DaemonStartOptions, commands.DaemonStopOptions, commands.DaemonStatusOptions, commands.DaemonLogsOptions, error) {
	var startOpts commands.DaemonStartOptions
	var stopOpts commands.DaemonStopOptions
	var statusOpts commands.DaemonStatusOptions
	logsOpts := commands.DaemonLogsOptions{Lines: 80}
	if len(args) == 0 {
		return "", startOpts, stopOpts, statusOpts, logsOpts, errors.New("daemon requires subcommand: start, stop, status, or logs")
	}
	switch args[0] {
	case "start":
		startOpts.Interval = 10 * time.Second
		for i := 1; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "--allow-provider-calls":
				startOpts.AllowProviderCalls = true
			case "--allow-overwrite":
				startOpts.AllowOverwrite = true
			case "--fail-fast":
				startOpts.FailFast = true
			case "--force":
				startOpts.Force = true
			case "--reset-log":
				startOpts.ResetLog = true
			case "--json":
				startOpts.JSON = true
			case "--interval":
				i++
				if i >= len(args) {
					return "", startOpts, stopOpts, statusOpts, logsOpts, errors.New("--interval requires a value")
				}
				duration, err := time.ParseDuration(args[i])
				if err != nil {
					return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--interval: invalid duration %q", args[i])
				}
				startOpts.Interval = duration
			case "--max-jobs":
				i++
				if i >= len(args) {
					return "", startOpts, stopOpts, statusOpts, logsOpts, errors.New("--max-jobs requires a value")
				}
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--max-jobs: invalid number %q", args[i])
				}
				startOpts.MaxJobs = n
			default:
				if value, ok := strings.CutPrefix(arg, "--interval="); ok {
					duration, err := time.ParseDuration(value)
					if err != nil {
						return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--interval: invalid duration %q", value)
					}
					startOpts.Interval = duration
					continue
				}
				if value, ok := strings.CutPrefix(arg, "--max-jobs="); ok {
					n, err := strconv.Atoi(value)
					if err != nil {
						return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--max-jobs: invalid number %q", value)
					}
					startOpts.MaxJobs = n
					continue
				}
				return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("unknown flag %q for daemon start", arg)
			}
		}
		return "start", startOpts, stopOpts, statusOpts, logsOpts, nil
	case "stop":
		for _, arg := range args[1:] {
			switch arg {
			case "--force":
				stopOpts.Force = true
			case "--json":
				stopOpts.JSON = true
			default:
				return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("unknown flag %q for daemon stop", arg)
			}
		}
		return "stop", startOpts, stopOpts, statusOpts, logsOpts, nil
	case "status":
		for _, arg := range args[1:] {
			switch arg {
			case "--json":
				statusOpts.JSON = true
			default:
				return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("unknown flag %q for daemon status", arg)
			}
		}
		return "status", startOpts, stopOpts, statusOpts, logsOpts, nil
	case "logs":
		for i := 1; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "--json":
				logsOpts.JSON = true
			case "--lines":
				i++
				if i >= len(args) {
					return "", startOpts, stopOpts, statusOpts, logsOpts, errors.New("--lines requires a value")
				}
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--lines: invalid number %q", args[i])
				}
				logsOpts.Lines = n
			default:
				if value, ok := strings.CutPrefix(arg, "--lines="); ok {
					n, err := strconv.Atoi(value)
					if err != nil {
						return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("--lines: invalid number %q", value)
					}
					logsOpts.Lines = n
					continue
				}
				return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("unknown flag %q for daemon logs", arg)
			}
		}
		return "logs", startOpts, stopOpts, statusOpts, logsOpts, nil
	default:
		return "", startOpts, stopOpts, statusOpts, logsOpts, fmt.Errorf("unknown daemon subcommand %q", args[0])
	}
}

func parseQueueArgs(args []string) (string, commands.QueueOptions, commands.QueueHealthOptions, error) {
	queueOpts := commands.QueueOptions{Limit: 10}
	healthOpts := commands.QueueHealthOptions{StaleAfter: 30 * time.Minute}
	if len(args) == 0 {
		return "queue", queueOpts, healthOpts, nil
	}
	if args[0] == "health" {
		for i := 1; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "--json":
				healthOpts.JSON = true
			case "--strict":
				healthOpts.Strict = true
			case "--write-report":
				healthOpts.WriteReport = true
			case "--stale-after":
				i++
				if i >= len(args) {
					return "", queueOpts, healthOpts, errors.New("--stale-after requires a value")
				}
				duration, err := time.ParseDuration(args[i])
				if err != nil {
					return "", queueOpts, healthOpts, fmt.Errorf("--stale-after: invalid duration %q", args[i])
				}
				healthOpts.StaleAfter = duration
			default:
				if value, ok := strings.CutPrefix(arg, "--stale-after="); ok {
					duration, err := time.ParseDuration(value)
					if err != nil {
						return "", queueOpts, healthOpts, fmt.Errorf("--stale-after: invalid duration %q", value)
					}
					healthOpts.StaleAfter = duration
					continue
				}
				return "", queueOpts, healthOpts, fmt.Errorf("unknown flag %q for queue health", arg)
			}
		}
		return "health", queueOpts, healthOpts, nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			queueOpts.JSON = true
		case "--failed":
			queueOpts.FailedOnly = true
		case "--approval-needed":
			queueOpts.ApprovalNeeded = true
		case "--running":
			queueOpts.RunningOnly = true
		case "--limit":
			i++
			if i >= len(args) {
				return "", queueOpts, healthOpts, errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return "", queueOpts, healthOpts, fmt.Errorf("--limit: invalid number %q", args[i])
			}
			queueOpts.Limit = n
		default:
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				n, err := strconv.Atoi(value)
				if err != nil {
					return "", queueOpts, healthOpts, fmt.Errorf("--limit: invalid number %q", value)
				}
				queueOpts.Limit = n
				continue
			}
			return "", queueOpts, healthOpts, fmt.Errorf("unknown flag %q for queue", arg)
		}
	}
	return "queue", queueOpts, healthOpts, nil
}

func parseAgentPlanArgs(args []string) (commands.AgentPlanCommandOptions, error) {
	var opts commands.AgentPlanCommandOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-review":
			opts.WriteReview = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--dry-run":
			opts.DryRun = true
		case "--planner-fallback-deterministic":
			opts.FallbackDeterministic = true
		case "--goal", "--input", "--make-id", "--creative-plan-id", "--run-id", "--platform", "--style-dir",
			"--planner", "--planner-model", "--planner-backend", "--planner-route",
			"--planner-timeout-seconds", "--planner-temperature", "--planner-max-output-chars":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			switch arg {
			case "--goal":
				opts.Goal = args[i]
			case "--input":
				opts.InputPath = args[i]
			case "--make-id":
				opts.MakeID = args[i]
			case "--creative-plan-id":
				opts.CreativePlanID = args[i]
			case "--run-id":
				opts.RunID = args[i]
			case "--platform":
				opts.Platform = args[i]
			case "--style-dir":
				opts.StyleDir = args[i]
			case "--planner":
				opts.PlannerName = args[i]
			case "--planner-model":
				opts.PlannerModel = args[i]
			case "--planner-backend":
				opts.PlannerBackend = args[i]
			case "--planner-route":
				opts.PlannerRoute = args[i]
			case "--planner-timeout-seconds":
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return opts, fmt.Errorf("--planner-timeout-seconds: invalid number %q", args[i])
				}
				opts.PlannerTimeoutSeconds = n
			case "--planner-temperature":
				f, err := strconv.ParseFloat(args[i], 64)
				if err != nil {
					return opts, fmt.Errorf("--planner-temperature: invalid number %q", args[i])
				}
				opts.PlannerTemperature = f
			case "--planner-max-output-chars":
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return opts, fmt.Errorf("--planner-max-output-chars: invalid number %q", args[i])
				}
				opts.PlannerMaxOutputChars = n
			}
		default:
			for _, prefix := range []string{"--goal=", "--input=", "--make-id=", "--creative-plan-id=", "--run-id=", "--platform=", "--style-dir=",
				"--planner=", "--planner-model=", "--planner-backend=", "--planner-route="} {
				if value, ok := strings.CutPrefix(arg, prefix); ok {
					switch prefix {
					case "--goal=":
						opts.Goal = value
					case "--input=":
						opts.InputPath = value
					case "--make-id=":
						opts.MakeID = value
					case "--creative-plan-id=":
						opts.CreativePlanID = value
					case "--run-id=":
						opts.RunID = value
					case "--platform=":
						opts.Platform = value
					case "--style-dir=":
						opts.StyleDir = value
					case "--planner=":
						opts.PlannerName = value
					case "--planner-model=":
						opts.PlannerModel = value
					case "--planner-backend=":
						opts.PlannerBackend = value
					case "--planner-route=":
						opts.PlannerRoute = value
					}
					goto nextArg
				}
			}
			return opts, fmt.Errorf("unknown agent-plan flag %q", arg)
		}
	nextArg:
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return opts, errors.New("agent-plan requires --goal")
	}
	return opts, nil
}

func parseAgentPlansArgs(args []string) (commands.AgentPlansListOptions, error) {
	opts := commands.AgentPlansListOptions{Limit: 20}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--status":
			i++
			if i >= len(args) {
				return opts, errors.New("--status requires a value")
			}
			opts.Status = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, fmt.Errorf("--limit: invalid number %q", args[i])
			}
			opts.Limit = n
		default:
			if value, ok := strings.CutPrefix(arg, "--status="); ok {
				opts.Status = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				n, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("--limit: invalid number %q", value)
				}
				opts.Limit = n
				continue
			}
			return opts, fmt.Errorf("unknown agent-plans flag %q", arg)
		}
	}
	return opts, nil
}

func parseInspectAgentPlanArgs(args []string) (string, commands.InspectAgentPlanCommandOptions, error) {
	var opts commands.InspectAgentPlanCommandOptions
	if len(args) == 0 {
		return "", opts, errors.New("inspect-agent-plan requires exactly one plan id")
	}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown inspect-agent-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("inspect-agent-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("inspect-agent-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseReviewAgentPlanArgs(args []string) (string, commands.ReviewAgentPlanCommandOptions, error) {
	var opts commands.ReviewAgentPlanCommandOptions
	if len(args) == 0 {
		return "", opts, errors.New("review-agent-plan requires exactly one plan id")
	}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown review-agent-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-agent-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-agent-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseAgentPolicyArgs(args []string) (string, commands.AgentPolicyCommandOptions, error) {
	var opts commands.AgentPolicyCommandOptions
	if len(args) == 0 {
		return "", opts, errors.New("agent-policy requires exactly one plan id")
	}
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-policy flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("agent-policy requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("agent-policy requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseApproveAgentPlanArgs(args []string) (string, commands.ApproveAgentPlanOptions, error) {
	var opts commands.ApproveAgentPlanOptions
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown approve-agent-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("approve-agent-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("approve-agent-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseRejectAgentPlanArgs(args []string) (string, commands.RejectAgentPlanOptions, error) {
	var opts commands.RejectAgentPlanOptions
	planID := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--reason":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--reason requires a value")
			}
			opts.Reason = args[i]
		default:
			if value, ok := strings.CutPrefix(arg, "--reason="); ok {
				opts.Reason = value
				continue
			}
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown reject-agent-plan flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("reject-agent-plan requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("reject-agent-plan requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseAgentPlanToJobArgs(args []string) (string, commands.AgentPlanToJobOptions, error) {
	var opts commands.AgentPlanToJobOptions
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--yes":
			opts.Yes = true
		case "--json":
			opts.JSON = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--approve-jobs":
			opts.ApproveJobs = true
		case "--force":
			opts.Force = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-plan-to-job flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("agent-plan-to-job requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("agent-plan-to-job requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseAgentPlanJobsArgs(args []string) (string, commands.AgentPlanJobsOptions, error) {
	var opts commands.AgentPlanJobsOptions
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-plan-jobs flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("agent-plan-jobs requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("agent-plan-jobs requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseAgentRunArgs(args []string) (string, commands.AgentRunOptions, error) {
	var opts commands.AgentRunOptions
	planID := ""
	for _, arg := range args {
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--convert":
			opts.Convert = true
		case "--approve-jobs":
			opts.ApproveJobs = true
		case "--run-jobs":
			opts.RunJobs = true
		case "--worker-once":
			opts.WorkerOnce = true
		case "--start-daemon":
			opts.StartDaemon = true
		case "--dry-run":
			opts.DryRun = true
		case "--json":
			opts.JSON = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--force":
			opts.Force = true
		case "--fail-fast":
			opts.FailFast = true
		case "--write-summary":
			opts.WriteSummary = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-run flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("agent-run requires exactly one plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("agent-run requires exactly one plan id")
	}
	return planID, opts, nil
}

func parseAgentGraphRunArgs(args []string) (string, commands.AgentGraphRunOptions, error) {
	var opts commands.AgentGraphRunOptions
	var planID string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--dry-run":
			opts.DryRun = true
		case "--workers-dir":
			i++
			if i >= len(args) {
				return "", opts, fmt.Errorf("--workers-dir requires a value")
			}
			opts.WorkersDir = args[i]
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown agent-graph-run flag %q", arg)
			}
			if planID != "" {
				return "", opts, fmt.Errorf("unexpected argument %q", arg)
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, fmt.Errorf("agent-graph-run requires a <plan_id> argument")
	}
	return planID, opts, nil
}

func parseAgentOrchestrateArgs(args []string) (commands.AgentOrchestrateOptions, error) {
	var extraWorkersDir string
	var skipGraph bool
	filtered := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--workers-dir":
			i++
			if i >= len(args) {
				return commands.AgentOrchestrateOptions{}, fmt.Errorf("--workers-dir requires a value")
			}
			extraWorkersDir = args[i]
		case "--skip-graph":
			skipGraph = true
		default:
			if value, ok := strings.CutPrefix(arg, "--workers-dir="); ok {
				extraWorkersDir = value
				continue
			}
			filtered = append(filtered, arg)
		}
	}
	planOpts, err := parseAgentPlanArgs(filtered)
	if err != nil {
		return commands.AgentOrchestrateOptions{}, err
	}
	return commands.AgentOrchestrateOptions{
		AgentPlanCommandOptions: planOpts,
		WorkersDir:              extraWorkersDir,
		SkipGraph:               skipGraph,
	}, nil
}

func parseVisualRequestsArgs(args []string) (string, commands.VisualRequestsOptions, error) {
	var opts commands.VisualRequestsOptions
	var planID string
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--overwrite":
			opts.Overwrite = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown visual-requests flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("visual-requests requires exactly one agent plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("visual-requests requires exactly one agent plan id")
	}
	return planID, opts, nil
}

func parseExecuteVisualRequestsArgs(args []string) (string, commands.ExecuteVisualRequestsOptions, error) {
	var opts commands.ExecuteVisualRequestsOptions
	var planID string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--yes":
			opts.Yes = true
		case "--json":
			opts.JSON = true
		case "--overwrite":
			opts.Overwrite = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-external-network":
			opts.AllowExternalNetwork = true
		case "--request-id":
			i++
			if i >= len(args) {
				return "", opts, errors.New("--request-id requires a value")
			}
			opts.RequestID = args[i]
		default:
			if value, ok := strings.CutPrefix(arg, "--request-id="); ok {
				opts.RequestID = value
				continue
			}
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown execute-visual-requests flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("execute-visual-requests requires exactly one agent plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("execute-visual-requests requires exactly one agent plan id")
	}
	return planID, opts, nil
}

func parseReviewVisualGenerationArgs(args []string) (string, commands.ReviewVisualGenerationOptions, error) {
	var opts commands.ReviewVisualGenerationOptions
	var planID string
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown review-visual-generation flag %q", arg)
			}
			if planID != "" {
				return "", opts, errors.New("review-visual-generation requires exactly one agent plan id")
			}
			planID = arg
		}
	}
	if planID == "" {
		return "", opts, errors.New("review-visual-generation requires exactly one agent plan id")
	}
	return planID, opts, nil
}

func parseCreateArgs(args []string) (commands.CreateOptions, error) {
	var opts commands.CreateOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--dry-run":
			opts.DryRun = true
		case "--write-review":
			opts.WriteReview = true
		case "--planner-fallback-deterministic":
			opts.FallbackDeterministic = true
		case "--skip-graph":
			opts.SkipGraph = true
		case "--yes":
			opts.Yes = true
		case "--allow-overwrite":
			opts.AllowOverwrite = true
		case "--allow-provider-calls":
			opts.AllowProviderCalls = true
		case "--allow-external-network":
			opts.AllowExternalNetwork = true
		case "--approve-jobs":
			opts.ApproveJobs = true
		case "--convert":
			opts.Convert = true
		case "--run-jobs":
			opts.RunJobs = true
		case "--worker-once":
			opts.WorkerOnce = true
		case "--start-daemon":
			opts.StartDaemon = true
		case "--fail-fast":
			opts.FailFast = true
		case "--burn-captions":
			opts.BurnCaptions = true
		case "--allow-missing-captions":
			opts.AllowMissingCaptions = true
		case "--generate-script":
			opts.GenerateScript = true
		case "--generate-captions":
			opts.GenerateCaptions = true
		case "--prepare-voiceover":
			opts.PrepareVoiceover = true
		case "--generate-voiceover":
			opts.GenerateVoiceover = true
		case "--mix-voiceover":
			opts.MixVoiceover = true
		case "--goal", "--input", "--planner", "--planner-model", "--workers-dir", "--approval-scope", "--platform", "--caption-position", "--caption-style":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			switch arg {
			case "--goal":
				opts.Goal = args[i]
			case "--input":
				opts.InputPath = args[i]
			case "--planner":
				opts.PlannerName = args[i]
			case "--planner-model":
				opts.PlannerModel = args[i]
			case "--workers-dir":
				opts.WorkersDir = args[i]
			case "--approval-scope":
				opts.ApprovalScope = args[i]
			case "--platform":
				opts.Platform = args[i]
			case "--caption-position":
				opts.CaptionPosition = args[i]
			case "--caption-style":
				opts.CaptionStyle = args[i]
			}
		default:
			if value, ok := strings.CutPrefix(arg, "--goal="); ok {
				opts.Goal = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--input="); ok {
				opts.InputPath = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--approval-scope="); ok {
				opts.ApprovalScope = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--platform="); ok {
				opts.Platform = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--workers-dir="); ok {
				opts.WorkersDir = value
				continue
			}
			if strings.HasPrefix(arg, "--") {
				return opts, fmt.Errorf("unknown create flag %q", arg)
			}
			if opts.InputPath != "" {
				return opts, errors.New("create accepts at most one positional input")
			}
			opts.InputPath = arg
		}
	}
	if strings.TrimSpace(opts.Goal) == "" {
		return opts, errors.New("create requires --goal")
	}
	if opts.ApprovalScope != "" {
		switch opts.ApprovalScope {
		case "preview", "local", "provider", "full":
		default:
			return opts, fmt.Errorf("invalid --approval-scope %q", opts.ApprovalScope)
		}
	}
	return opts, nil
}

func parseCreateResultArgs(args []string) (string, commands.CreateResultOptions, error) {
	var opts commands.CreateResultOptions
	var sessionID string
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--write-artifact":
			opts.WriteArtifact = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown create-result flag %q", arg)
			}
			if sessionID != "" {
				return "", opts, errors.New("create-result requires exactly one session id")
			}
			sessionID = arg
		}
	}
	if sessionID == "" {
		return "", opts, errors.New("create-result requires exactly one session id")
	}
	return sessionID, opts, nil
}

func parseCreateSessionsArgs(args []string) (commands.CreateSessionsOptions, error) {
	opts := commands.CreateSessionsOptions{Limit: 20}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--status":
			i++
			if i >= len(args) {
				return opts, errors.New("--status requires a value")
			}
			opts.Status = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return opts, errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, fmt.Errorf("--limit: invalid number %q", args[i])
			}
			opts.Limit = n
		default:
			if value, ok := strings.CutPrefix(arg, "--status="); ok {
				opts.Status = value
				continue
			}
			if value, ok := strings.CutPrefix(arg, "--limit="); ok {
				n, err := strconv.Atoi(value)
				if err != nil {
					return opts, fmt.Errorf("--limit: invalid number %q", value)
				}
				opts.Limit = n
				continue
			}
			return opts, fmt.Errorf("unknown create-sessions flag %q", arg)
		}
	}
	return opts, nil
}

func parseInspectCreateSessionArgs(args []string) (string, commands.InspectCreateSessionOptions, error) {
	var opts commands.InspectCreateSessionOptions
	var sessionID string
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.JSON = true
		default:
			if strings.HasPrefix(arg, "--") {
				return "", opts, fmt.Errorf("unknown inspect-create-session flag %q", arg)
			}
			if sessionID != "" {
				return "", opts, errors.New("inspect-create-session requires exactly one session id")
			}
			sessionID = arg
		}
	}
	if sessionID == "" {
		return "", opts, errors.New("inspect-create-session requires exactly one session id")
	}
	return sessionID, opts, nil
}

func parsePlannerDiagnoseArgs(args []string) (commands.PlannerDiagnoseOptions, error) {
	var opts commands.PlannerDiagnoseOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--check":
			opts.Check = true
		case "--json":
			opts.JSON = true
		case "--planner-fallback-deterministic":
			// accepted and silently ignored in diagnose context
		case "--planner", "--planner-model", "--planner-backend", "--planner-route", "--planner-timeout-seconds":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			switch arg {
			case "--planner":
				opts.PlannerName = args[i]
			case "--planner-model":
				opts.PlannerModel = args[i]
			case "--planner-backend":
				opts.PlannerBackend = args[i]
			case "--planner-route":
				opts.PlannerRoute = args[i]
			case "--planner-timeout-seconds":
				n, err := strconv.Atoi(args[i])
				if err != nil {
					return opts, fmt.Errorf("--planner-timeout-seconds: invalid number %q", args[i])
				}
				opts.PlannerTimeoutSeconds = n
			}
		default:
			if strings.HasPrefix(arg, "--") {
				return opts, fmt.Errorf("unknown agent-planner-diagnose flag %q", arg)
			}
		}
	}
	return opts, nil
}
