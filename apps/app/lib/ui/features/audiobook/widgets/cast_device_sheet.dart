import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../data/models/book.dart';
import '../../../../data/models/cast_models.dart';
import '../../../core/tokens.dart';
import '../../../core/typography.dart';
import '../../../state/audio_player_state.dart';
import '../../../state/cast_provider.dart';
import '../../../state/providers.dart';

Future<void> showCastDeviceSheet(
  BuildContext context, {
  Book? book,
  bool hasReadalong = false,
}) {
  return showModalBottomSheet<void>(
    context: context,
    backgroundColor: AppTokens.obsidianBackground,
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
    ),
    builder: (ctx) => CastDeviceSheet(
      book: book,
      hasReadalong: hasReadalong,
    ),
  );
}

class CastDeviceSheet extends ConsumerStatefulWidget {
  final Book? book;
  final bool hasReadalong;

  const CastDeviceSheet({
    super.key,
    this.book,
    this.hasReadalong = false,
  });

  @override
  ConsumerState<CastDeviceSheet> createState() => _CastDeviceSheetState();
}

class _CastDeviceSheetState extends ConsumerState<CastDeviceSheet> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(castProvider.notifier).refreshDevices(refresh: true);
    });
  }

  Future<void> _selectDevice(CastDevice device) async {
    final playerState = ref.read(audioPlayerProvider);
    if (playerState.book == null && widget.book != null) {
      await ref.read(audioPlayerProvider.notifier).loadBook(
            widget.book!,
            autoPlay: false,
          );
      if (widget.hasReadalong) {
        ref
            .read(audioPlayerProvider.notifier)
            .setPlaybackMode(AudioPlaybackMode.readalong);
      }
    }
    await ref.read(castProvider.notifier).connectToDevice(device);
    if (mounted) {
      Navigator.of(context).pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final castState = ref.watch(castProvider);
    final playerState = ref.watch(audioPlayerProvider);
    final activeDevice = castState.activeDevice ?? playerState.activeCastDevice;
    final session = castState.session;
    final effectiveHasReadalong =
        widget.hasReadalong || (playerState.book?.hasReadalong ?? false);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          AppTokens.space24,
          AppTokens.space16,
          AppTokens.space24,
          AppTokens.space24,
        ),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const Icon(Icons.cast, color: AppTokens.amberAccent, size: 22),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Cast Audio',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.titleSerif(
                        fontSize: 20,
                        color: Colors.white,
                      ),
                    ),
                  ),
                  IconButton(
                    key: const Key('cast_sheet_refresh_button'),
                    tooltip: 'Refresh Cast devices',
                    icon: castState.isDiscovering
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: AppTokens.amberAccent,
                            ),
                          )
                        : const Icon(Icons.refresh, color: AppTokens.antiqueCream),
                    onPressed: castState.isDiscovering
                        ? null
                        : () => ref
                            .read(castProvider.notifier)
                            .refreshDevices(refresh: true),
                  ),
                ],
              ),
              if (castState.errorMessage != null) ...[
                const SizedBox(height: 8),
                Text(
                  castState.errorMessage!,
                  style: AppTypography.bodySans(
                    fontSize: 12,
                    color: Colors.redAccent.shade100,
                  ),
                ),
              ],
              const SizedBox(height: AppTokens.space12),

              // Local device output option
              Container(
                decoration: BoxDecoration(
                  color: activeDevice == null
                      ? AppTokens.amberAccent.withValues(alpha: 0.12)
                      : Colors.white.withValues(alpha: 0.04),
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                ),
                child: Material(
                  type: MaterialType.transparency,
                  child: ListTile(
                    key: const Key('cast_sheet_this_device_tile'),
                    leading: Icon(
                      Icons.headphones_rounded,
                      color: activeDevice == null
                          ? AppTokens.amberAccent
                          : AppTokens.antiqueCream.withValues(alpha: 0.7),
                    ),
                    title: Text(
                      'This device',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        fontWeight: activeDevice == null
                            ? FontWeight.w700
                            : FontWeight.w500,
                        color: activeDevice == null
                            ? AppTokens.amberAccent
                            : Colors.white,
                      ),
                    ),
                    trailing: activeDevice == null
                        ? const Icon(Icons.check, color: AppTokens.amberAccent)
                        : null,
                    onTap: () async {
                      if (activeDevice != null) {
                        await ref.read(castProvider.notifier).disconnectToLocal();
                      }
                      if (context.mounted) Navigator.of(context).pop();
                    },
                  ),
                ),
              ),

              const SizedBox(height: AppTokens.space12),

              if (castState.devices.isEmpty && !castState.isDiscovering)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: AppTokens.space16),
                  child: Text(
                    'No Chromecast devices found on the local network. Ensure Shelfd is on the same LAN or configure SHELFD_CAST_STATIC_DEVICES.',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: AppTokens.antiqueCream.withValues(alpha: 0.65),
                    ),
                  ),
                ),

              for (final device in castState.devices) ...[
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Container(
                    decoration: BoxDecoration(
                      color: activeDevice?.id == device.id
                          ? AppTokens.amberAccent.withValues(alpha: 0.12)
                          : Colors.white.withValues(alpha: 0.04),
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    ),
                    child: Material(
                      type: MaterialType.transparency,
                      child: ListTile(
                        key: Key('cast_device_tile_${device.id}'),
                        leading: Icon(
                          device.hasVideo ? Icons.tv_rounded : Icons.speaker_rounded,
                          color: activeDevice?.id == device.id
                              ? AppTokens.amberAccent
                              : AppTokens.antiqueCream.withValues(alpha: 0.8),
                        ),
                        title: Text(
                          device.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            fontWeight: activeDevice?.id == device.id
                                ? FontWeight.w700
                                : FontWeight.w500,
                            color: activeDevice?.id == device.id
                                ? AppTokens.amberAccent
                                : Colors.white,
                          ),
                        ),
                        subtitle: device.model.isNotEmpty
                            ? Text(
                                device.model,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.bodySans(
                                  fontSize: 12,
                                  color: AppTokens.antiqueCream.withValues(alpha: 0.6),
                                ),
                              )
                            : null,
                        trailing: activeDevice?.id == device.id
                            ? const Icon(Icons.cast_connected, color: AppTokens.amberAccent)
                            : null,
                        onTap: castState.isBusy ? null : () => _selectDevice(device),
                      ),
                    ),
                  ),
                ),
              ],

              if (activeDevice != null && session != null) ...[
                const Divider(color: Colors.white12, height: 24),

                if (activeDevice.hasVideo && effectiveHasReadalong)
                  Container(
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.04),
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    ),
                    child: Material(
                      type: MaterialType.transparency,
                      child: SwitchListTile(
                        key: const Key('cast_sheet_captions_toggle'),
                        title: Text(
                          'Show text captions on TV',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            color: Colors.white,
                          ),
                        ),
                        activeThumbColor: AppTokens.amberAccent,
                        value: session.captionsEnabled,
                        onChanged: (val) => ref
                            .read(castProvider.notifier)
                            .setCaptionsEnabled(val),
                      ),
                    ),
                  ),

                const SizedBox(height: AppTokens.space12),

                Row(
                  children: [
                    const Icon(Icons.volume_up_rounded, color: AppTokens.antiqueCream, size: 20),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Slider(
                        key: const Key('cast_sheet_volume_slider'),
                        value: session.volume.clamp(0.0, 1.0),
                        activeColor: AppTokens.amberAccent,
                        inactiveColor: Colors.white24,
                        onChanged: (v) =>
                            ref.read(castProvider.notifier).setCastVolume(v),
                      ),
                    ),
                  ],
                ),

                const SizedBox(height: AppTokens.space8),

                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    key: const Key('cast_sheet_stop_button'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: AppTokens.amberAccent,
                      side: const BorderSide(color: AppTokens.amberAccent),
                    ),
                    icon: const Icon(Icons.cast_connected, size: 18),
                    label: const Text('Stop Casting'),
                    onPressed: castState.isBusy
                        ? null
                        : () async {
                            await ref.read(castProvider.notifier).disconnectToLocal();
                            if (context.mounted) Navigator.of(context).pop();
                          },
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
