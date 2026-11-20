import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/background_job.dart';
import '../../../data/models/plugin.dart';
import '../../../data/services/api_service.dart';
import '../../core/file_saver/file_saver.dart';
import '../../state/providers.dart';
import 'plugin_action_dialog.dart';

class PluginActionRunner {
  PluginActionRunner._();

  static String _sanitizeFileName(String rawTitle, String defaultExt) {
    final trimmed = rawTitle.trim();
    final sanitized = trimmed
        .replaceAll(RegExp(r'[\\/:*?"<>|\x00-\x1f]'), '_')
        .trim();
    final base = sanitized.isNotEmpty ? sanitized : 'book';
    final ext = defaultExt.startsWith('.')
        ? defaultExt.substring(1)
        : defaultExt;
    return '$base.$ext';
  }

  static Future<PluginActionResult?> runPluginAction(
    BuildContext context,
    WidgetRef ref, {
    required PluginSummary plugin,
    required PluginActionDef action,
    required PluginActionTarget target,
  }) async {
    try {
      PluginActionResult? result;
      if (action.isForm) {
        result = await showPluginActionDialog(
          context,
          plugin: plugin,
          action: action,
          target: target,
        );
        if (result == null) return null;
      } else {
        final api = ref.read(apiServiceProvider);
        result = await api.invokePluginAction(
          plugin.name,
          action.name,
          target: target,
        );
      }

      if (result.job != null) {
        ref.read(pluginJobsProvider.notifier).enqueueOrTrackJob(result.job!);
        ref
            .read(backgroundJobsProvider.notifier)
            .upsertJob(BackgroundJob.fromPluginJob(result.job!));
      } else if (result.isJobStarted && result.jobId != null) {
        final fallback = PluginJob(
          id: result.jobId!,
          plugin: plugin.name,
          action: action.name,
          title: target.title ?? action.title,
          targetType: target.type,
          targetId: target.id,
          status: 'running',
          message: result.message ?? 'Starting...',
        );
        ref.read(pluginJobsProvider.notifier).enqueueOrTrackJob(fallback);
        ref
            .read(backgroundJobsProvider.notifier)
            .upsertJob(BackgroundJob.fromPluginJob(fallback));
      }

      if (result.isClientHandoff) {
        if (result.downloadApiPath != null &&
            result.downloadApiPath!.trim().isNotEmpty) {
          final api = ref.read(apiServiceProvider);
          final bytes =
              await api.downloadApiPathBytes(result.downloadApiPath!.trim());
          final fileName =
              (result.fileName != null && result.fileName!.trim().isNotEmpty)
                  ? result.fileName!.trim()
                  : _sanitizeFileName(target.title ?? 'book', 'md');
          await saveDownloadedFile(
            bytes,
            fileName,
            mimeType: result.mimeType ?? 'text/markdown;charset=utf-8',
          );
        }
        final urlToOpen = result.openUrl ?? result.externalUrl;
        if (urlToOpen != null && urlToOpen.trim().isNotEmpty) {
          await openExternalUrl(urlToOpen.trim());
        }
      } else if (result.isCompleted) {
        final urlToOpen = result.openUrl ?? result.externalUrl;
        if (urlToOpen != null && urlToOpen.trim().isNotEmpty) {
          await openExternalUrl(urlToOpen.trim());
        }
      }

      if (context.mounted) {
        final msg = (result.message != null && result.message!.trim().isNotEmpty)
            ? result.message!.trim()
            : 'Completed ${action.title}';
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(msg),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
      return result;
    } catch (e) {
      if (context.mounted) {
        final msg = e is ApiException ? e.message : e.toString();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to run ${action.title}: $msg'),
            backgroundColor: Colors.red.shade800,
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
      return null;
    }
  }
}
