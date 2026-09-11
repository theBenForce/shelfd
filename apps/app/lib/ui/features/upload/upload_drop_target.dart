import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'directory_scanner.dart';

/// Helper to navigate to the dedicated Uploads & Ingestion page.
Future<void> pickAndUploadEpub(BuildContext context, WidgetRef ref) async {
  context.go('/uploads');
}

/// A wrapper widget that listens for drag-and-dropped EPUB files and displays
/// the Warm Editorial dashed drop target overlay when files hover over the app.
class ShelfdDropTarget extends ConsumerStatefulWidget {
  final Widget child;

  const ShelfdDropTarget({super.key, required this.child});

  @override
  ConsumerState<ShelfdDropTarget> createState() => _ShelfdDropTargetState();
}

class _ShelfdDropTargetState extends ConsumerState<ShelfdDropTarget> {
  bool _isDragging = false;

  @override
  Widget build(BuildContext context) {
    return DropTarget(
      onDragEntered: (_) => setState(() => _isDragging = true),
      onDragExited: (_) => setState(() => _isDragging = false),
      onDragDone: (details) async {
        setState(() => _isDragging = false);
        if (details.files.isEmpty) return;

        final pickedList = <PickedEpubFile>[];

        for (final item in details.files) {
          final path = item.path;
          if (path.isNotEmpty && isDirectoryPath(path)) {
            final nestedEpubs = await scanPathForEpubs(path);
            pickedList.addAll(nestedEpubs);
          } else if (item.name.toLowerCase().endsWith('.epub')) {
            pickedList.add(PickedEpubFile(
              name: item.name,
              path: item.path.isNotEmpty ? item.path : null,
              readBytes: () => item.readAsBytes(),
            ));
          }
        }

        if (pickedList.isEmpty) {
          if (context.mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('No valid .epub files detected in dropped items')),
            );
          }
          return;
        }

        if (context.mounted) {
          final count = await ref.read(uploadProvider.notifier).uploadEpubFiles(pickedList);
          if (context.mounted) {
            final currentPath = GoRouterState.of(context).uri.path;
            if (currentPath != '/uploads') {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text('Staged $count book(s) for review'),
                  backgroundColor: AppTokens.charcoalInk,
                  action: SnackBarAction(
                    label: 'View in Uploads',
                    textColor: const Color(0xFFFFD43B),
                    onPressed: () => context.go('/uploads'),
                  ),
                ),
              );
            }
          }
        }
      },
      child: Stack(
        children: [
          widget.child,
          if (_isDragging)
            Positioned.fill(
              child: Container(
                color: Colors.black.withValues(alpha: 0.45),
                padding: const EdgeInsets.all(AppTokens.space24),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 520),
                    child: Container(
                      padding: const EdgeInsets.all(AppTokens.space32),
                      decoration: BoxDecoration(
                        color: AppTokens.boneSurface,
                        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
                        border: Border.all(
                          color: AppTokens.charcoalInk,
                          width: 2,
                        ),
                        boxShadow: const [
                          BoxShadow(
                            color: Color(0x2A000000),
                            blurRadius: 24,
                            offset: Offset(0, 12),
                          ),
                        ],
                      ),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Container(
                            width: 64,
                            height: 64,
                            decoration: BoxDecoration(
                              color: AppTokens.boneContainer,
                              shape: BoxShape.circle,
                              border: Border.all(color: AppTokens.crispBorder),
                            ),
                            child: const Icon(
                              Icons.cloud_upload_outlined,
                              size: 32,
                              color: AppTokens.charcoalInk,
                            ),
                          ),
                          const SizedBox(height: AppTokens.space16),
                          Text(
                            'Drop EPUB anywhere to upload',
                            textAlign: TextAlign.center,
                            style: AppTypography.titleSerif(
                              fontSize: 22,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: AppTokens.space8),
                          Text(
                            'Supports standard .epub format. Metadata can be reviewed and edited before saving to your library.',
                            textAlign: TextAlign.center,
                            style: AppTypography.bodySans(
                              fontSize: 13,
                              color: AppTokens.mutedCopy,
                            ),
                          ),
                          const SizedBox(height: AppTokens.space20),
                          Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: const [
                              _FormatBadge(label: '.EPUB'),
                              SizedBox(width: AppTokens.space8),
                              _FormatBadge(label: 'EPUB 3'),
                              SizedBox(width: AppTokens.space8),
                              _FormatBadge(label: 'EPUB 2'),
                            ],
                          ),
                          const SizedBox(height: AppTokens.space24),
                          OutlinedButton.icon(
                            style: OutlinedButton.styleFrom(
                              foregroundColor: AppTokens.charcoalInk,
                              side: const BorderSide(color: AppTokens.crispBorder),
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppTokens.space20,
                                vertical: AppTokens.space12,
                              ),
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              ),
                            ),
                            onPressed: () {
                              setState(() => _isDragging = false);
                              pickAndUploadEpub(context, ref);
                            },
                            icon: const Icon(Icons.folder_open_rounded, size: 18),
                            label: const Text(
                              'Browse Local Files',
                              style: TextStyle(fontWeight: FontWeight.w600),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _FormatBadge extends StatelessWidget {
  final String label;

  const _FormatBadge({required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space8,
        vertical: AppTokens.space4,
      ),
      decoration: BoxDecoration(
        color: AppTokens.boneContainer,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(color: AppTokens.crispBorder),
      ),
      child: Text(
        label,
        style: const TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: AppTokens.mutedCopy,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}
