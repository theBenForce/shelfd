import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/yoto.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

Future<YotoDisplayIcon?> showYotoIconPickerDialog(
  BuildContext context, {
  String? currentIconUri,
  String? targetLabel,
}) {
  return showDialog<YotoDisplayIcon>(
    context: context,
    builder: (ctx) => YotoIconPickerDialog(
      currentIconUri: currentIconUri,
      targetLabel: targetLabel,
    ),
  );
}

class YotoIconPickerDialog extends ConsumerStatefulWidget {
  final String? currentIconUri;
  final String? targetLabel;

  const YotoIconPickerDialog({
    super.key,
    this.currentIconUri,
    this.targetLabel,
  });

  @override
  ConsumerState<YotoIconPickerDialog> createState() =>
      _YotoIconPickerDialogState();
}

class _YotoIconPickerDialogState extends ConsumerState<YotoIconPickerDialog> {
  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';
  String _activeTab = 'public'; // 'public' or 'custom'
  bool _isUploading = false;
  String? _uploadError;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _handleUploadCustomIcon() async {
    setState(() {
      _uploadError = null;
    });

    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['png', 'jpg', 'jpeg', 'gif', 'webp'],
      );
      if (result.isEmpty) return;

      final file = result.first;
      final bytes = await file.readAsBytes();
      if (bytes.isEmpty) {
        setState(() {
          _uploadError = 'Could not read image bytes.';
        });
        return;
      }

      setState(() {
        _isUploading = true;
      });

      final repo = ref.read(yotoRepositoryProvider);
      final uploadedIcon = await repo.uploadCustomIcon(
        filename: file.name.isNotEmpty ? file.name : 'custom-icon.png',
        bytes: bytes,
      );

      ref.invalidate(yotoIconsProvider);
      if (mounted) {
        Navigator.of(context).pop(uploadedIcon);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isUploading = false;
          _uploadError = 'Failed to upload 16x16 icon: $e';
        });
      }
    }
  }

  List<YotoDisplayIcon> _filterIcons(List<YotoDisplayIcon> icons) {
    final q = _searchQuery.trim().toLowerCase();
    if (q.isEmpty) return icons;
    return icons.where((icon) {
      if (icon.title.toLowerCase().contains(q)) return true;
      for (final tag in icon.publicTags) {
        if (tag.toLowerCase().contains(q)) return true;
      }
      return false;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final iconsAsync = ref.watch(yotoIconsProvider);

    return Dialog(
      key: const Key('yoto_icon_picker_dialog'),
      backgroundColor: colors.surface,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: BorderSide(color: colors.border),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: 560,
          maxHeight: 620,
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.grid_view_rounded,
                    size: 20,
                    color: colors.accentPrimary,
                  ),
                  const SizedBox(width: AppTokens.space8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Select 16×16 Yoto Display Icon',
                          style: AppTypography.titleSerif(
                            fontSize: 18,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        if (widget.targetLabel != null &&
                            widget.targetLabel!.isNotEmpty)
                          Text(
                            widget.targetLabel!,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.captionSans(
                              color: colors.secondaryInk,
                            ),
                          ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, color: colors.secondaryInk),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Row(
                children: [
                  Expanded(
                    child: SegmentedButton<String>(
                      style: ButtonStyle(
                        backgroundColor: WidgetStateProperty.resolveWith(
                          (states) => states.contains(WidgetState.selected)
                              ? colors.accentContainer
                              : colors.containerLow,
                        ),
                        foregroundColor: WidgetStateProperty.resolveWith(
                          (states) => states.contains(WidgetState.selected)
                              ? colors.accentPrimary
                              : colors.ink,
                        ),
                        side: WidgetStatePropertyAll(
                          BorderSide(color: colors.border),
                        ),
                      ),
                      segments: const [
                        ButtonSegment<String>(
                          value: 'public',
                          label: Text('Yoto Icons'),
                          icon: Icon(Icons.public, size: 16),
                        ),
                        ButtonSegment<String>(
                          value: 'custom',
                          label: Text('My Icons'),
                          icon: Icon(Icons.star_outline, size: 16),
                        ),
                      ],
                      selected: {_activeTab},
                      onSelectionChanged: (selection) {
                        if (selection.isNotEmpty) {
                          setState(() {
                            _activeTab = selection.first;
                          });
                        }
                      },
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  OutlinedButton.icon(
                    key: const Key('yoto_upload_custom_icon_button'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.ink,
                      side: BorderSide(color: colors.border),
                    ),
                    onPressed: _isUploading ? null : _handleUploadCustomIcon,
                    icon: _isUploading
                        ? SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: colors.accentPrimary,
                            ),
                          )
                        : const Icon(Icons.upload_rounded, size: 16),
                    label: Text(_isUploading ? 'Uploading...' : 'Upload Image'),
                  ),
                ],
              ),
              if (_uploadError != null) ...[
                const SizedBox(height: AppTokens.space8),
                Text(
                  _uploadError!,
                  style: AppTypography.captionSans(color: colors.errorText),
                ),
              ],
              const SizedBox(height: AppTokens.space12),
              TextField(
                key: const Key('yoto_icon_search_field'),
                controller: _searchController,
                onChanged: (val) => setState(() => _searchQuery = val),
                style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                decoration: InputDecoration(
                  hintText: 'Search icons by title or tag (e.g. book, music, star)...',
                  isDense: true,
                  prefixIcon: Icon(Icons.search, size: 18, color: colors.mutedInk),
                  suffixIcon: _searchQuery.isNotEmpty
                      ? IconButton(
                          icon: Icon(Icons.clear, size: 16, color: colors.mutedInk),
                          onPressed: () {
                            _searchController.clear();
                            setState(() => _searchQuery = '');
                          },
                        )
                      : null,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    borderSide: BorderSide(color: colors.border),
                  ),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    borderSide: BorderSide(color: colors.border),
                  ),
                ),
              ),
              const SizedBox(height: AppTokens.space16),
              Expanded(
                child: iconsAsync.when(
                  loading: () => Center(
                    child: CircularProgressIndicator(color: colors.accentPrimary),
                  ),
                  error: (err, _) => Center(
                    child: Text(
                      'Failed to load Yoto icons: $err',
                      textAlign: TextAlign.center,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        color: colors.errorText,
                      ),
                    ),
                  ),
                  data: (catalog) {
                    final sourceList = _activeTab == 'custom'
                        ? catalog.userIcons
                        : catalog.publicIcons;
                    final filtered = _filterIcons(sourceList);

                    if (filtered.isEmpty) {
                      return Center(
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(
                              Icons.image_not_supported_outlined,
                              size: 36,
                              color: colors.mutedInk,
                            ),
                            const SizedBox(height: AppTokens.space8),
                            Text(
                              _activeTab == 'custom'
                                  ? 'No custom icons found. Click "Upload Image" to convert any picture to a 16×16 Yoto icon.'
                                  : 'No matching Yoto icons found.',
                              textAlign: TextAlign.center,
                              style: AppTypography.bodySans(
                                fontSize: 13,
                                color: colors.secondaryInk,
                              ),
                            ),
                          ],
                        ),
                      );
                    }

                    return GridView.builder(
                      gridDelegate:
                          const SliverGridDelegateWithMaxCrossAxisExtent(
                        maxCrossAxisExtent: 96,
                        mainAxisSpacing: AppTokens.space8,
                        crossAxisSpacing: AppTokens.space8,
                        childAspectRatio: 0.85,
                      ),
                      itemCount: filtered.length,
                      itemBuilder: (context, index) {
                        final icon = filtered[index];
                        final isSelected = widget.currentIconUri != null &&
                            (widget.currentIconUri == icon.yotoUri ||
                                widget.currentIconUri == icon.mediaId);

                        return InkWell(
                          key: Key('yoto_icon_tile_${icon.mediaId}'),
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          onTap: () => Navigator.of(context).pop(icon),
                          child: Container(
                            padding: const EdgeInsets.all(AppTokens.space8),
                            decoration: BoxDecoration(
                              color: isSelected
                                  ? colors.accentContainer
                                  : colors.containerLow,
                              borderRadius:
                                  BorderRadius.circular(AppTokens.radiusSm),
                              border: Border.all(
                                color: isSelected
                                    ? colors.accentPrimary
                                    : colors.subtleBorder,
                                width: isSelected ? 2 : 1,
                              ),
                            ),
                            child: Column(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Container(
                                  width: 36,
                                  height: 36,
                                  decoration: BoxDecoration(
                                    color: AppTokens.obsidianBackground,
                                    borderRadius: BorderRadius.circular(4),
                                  ),
                                  alignment: Alignment.center,
                                  child: icon.url.isNotEmpty
                                      ? Image.network(
                                          icon.url,
                                          width: 28,
                                          height: 28,
                                          fit: BoxFit.contain,
                                          filterQuality: FilterQuality.none,
                                          errorBuilder: (_, _, _) => const Icon(
                                            Icons.auto_stories,
                                            size: 18,
                                            color: AppTokens.amberAccent,
                                          ),
                                        )
                                      : const Icon(
                                          Icons.auto_stories,
                                          size: 18,
                                          color: AppTokens.amberAccent,
                                        ),
                                ),
                                const SizedBox(height: AppTokens.space4),
                                Text(
                                  icon.title.isNotEmpty
                                      ? icon.title
                                      : (icon.publicTags.isNotEmpty
                                          ? icon.publicTags.first
                                          : 'Icon'),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  textAlign: TextAlign.center,
                                  style: AppTypography.captionSans(
                                    fontSize: 11,
                                    color: colors.ink,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        );
                      },
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
