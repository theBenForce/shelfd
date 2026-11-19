import 'dart:async';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/plugin.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

Future<PluginOptionItem?> showPluginAssetPickerDialog(
  BuildContext context, {
  required String pluginName,
  required String actionName,
  required PluginInputFieldSpec field,
  String? currentValue,
  String? targetLabel,
}) {
  return showDialog<PluginOptionItem>(
    context: context,
    builder: (ctx) => PluginAssetPickerDialog(
      pluginName: pluginName,
      actionName: actionName,
      field: field,
      currentValue: currentValue,
      targetLabel: targetLabel,
    ),
  );
}

class PluginAssetPickerDialog extends ConsumerStatefulWidget {
  final String pluginName;
  final String actionName;
  final PluginInputFieldSpec field;
  final String? currentValue;
  final String? targetLabel;

  const PluginAssetPickerDialog({
    super.key,
    required this.pluginName,
    required this.actionName,
    required this.field,
    this.currentValue,
    this.targetLabel,
  });

  @override
  ConsumerState<PluginAssetPickerDialog> createState() =>
      _PluginAssetPickerDialogState();
}

class _PluginAssetPickerDialogState
    extends ConsumerState<PluginAssetPickerDialog> {
  final TextEditingController _searchController = TextEditingController();
  Timer? _searchDebounce;
  String _searchQuery = '';
  late String _activeTab;
  List<PluginOptionItem> _items = const [];
  bool _isLoading = true;
  bool _isUploading = false;
  String? _errorMessage;

  List<PluginAssetPickerTab> get _tabs {
    if (widget.field.tabs.isNotEmpty) {
      return widget.field.tabs;
    }
    return const [
      PluginAssetPickerTab(id: 'public', label: 'Public Icons', searchable: true),
    ];
  }

  bool get _isSquarePixel => widget.field.assetShape == 'square_pixel';

  @override
  void initState() {
    super.initState();
    _activeTab = _tabs.first.id;
    _items = widget.field.options;
    if (widget.field.optionsSource != null &&
        widget.field.optionsSource!.isNotEmpty) {
      _loadOptions();
    } else {
      _isLoading = false;
    }
  }

  @override
  void dispose() {
    _searchDebounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _loadOptions() async {
    final source = widget.field.optionsSource;
    if (source == null || source.isEmpty) {
      setState(() {
        _items = widget.field.options;
        _isLoading = false;
      });
      return;
    }

    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    try {
      final api = ref.read(apiServiceProvider);
      final res = await api.getPluginActionOptions(
        widget.pluginName,
        widget.actionName,
        source: source,
        tab: _activeTab,
        query: _searchQuery.trim().isNotEmpty ? _searchQuery.trim() : null,
      );
      if (!mounted) return;
      setState(() {
        _items = res.items;
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _errorMessage = 'Failed to load assets: $e';
      });
    }
  }

  void _onSearchChanged(String val) {
    setState(() {
      _searchQuery = val;
    });
    _searchDebounce?.cancel();
    _searchDebounce = Timer(const Duration(milliseconds: 220), () {
      if (mounted &&
          widget.field.optionsSource != null &&
          widget.field.optionsSource!.isNotEmpty) {
        _loadOptions();
      }
    });
  }

  Future<void> _handleUploadCustomAsset() async {
    setState(() {
      _errorMessage = null;
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
        if (!mounted) return;
        setState(() {
          _errorMessage = 'Could not read image bytes.';
        });
        return;
      }

      if (!mounted) return;
      setState(() {
        _isUploading = true;
      });

      final api = ref.read(apiServiceProvider);
      final uploaded = await api.uploadPluginActionAsset(
        widget.pluginName,
        widget.actionName,
        filename: file.name.isNotEmpty ? file.name : 'custom-asset.png',
        bytes: bytes,
        field: widget.field.key,
      );

      if (mounted) {
        Navigator.of(context).pop(uploaded);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isUploading = false;
          _errorMessage = 'Failed to upload custom asset: $e';
        });
      }
    }
  }

  List<PluginOptionItem> _filteredItems() {
    final q = _searchQuery.trim().toLowerCase();
    if (q.isEmpty) return _items;
    return _items.where((item) {
      if (item.label.toLowerCase().contains(q)) return true;
      if (item.value.toLowerCase().contains(q)) return true;
      if (item.subtitle != null && item.subtitle!.toLowerCase().contains(q)) {
        return true;
      }
      return false;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final tabs = _tabs;
    final filtered = _filteredItems();

    return Dialog(
      key: const Key('plugin_asset_picker_dialog'),
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
                          widget.field.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
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
                  if (tabs.length > 1)
                    Expanded(
                      child: SegmentedButton<String>(
                        key: const Key('plugin_asset_tabs'),
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
                        segments: tabs
                            .map(
                              (t) => ButtonSegment<String>(
                                value: t.id,
                                label: Text(t.label),
                              ),
                            )
                            .toList(),
                        selected: {_activeTab},
                        onSelectionChanged: (selection) {
                          if (selection.isNotEmpty) {
                            setState(() {
                              _activeTab = selection.first;
                            });
                            _loadOptions();
                          }
                        },
                      ),
                    )
                  else
                    const Spacer(),
                  if (widget.field.allowUpload) ...[
                    const SizedBox(width: AppTokens.space12),
                    OutlinedButton.icon(
                      key: const Key('plugin_asset_upload_button'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: colors.ink,
                        side: BorderSide(color: colors.border),
                      ),
                      onPressed:
                          _isUploading ? null : _handleUploadCustomAsset,
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
                      label:
                          Text(_isUploading ? 'Uploading...' : 'Upload Image'),
                    ),
                  ],
                ],
              ),
              if (_errorMessage != null) ...[
                const SizedBox(height: AppTokens.space8),
                Text(
                  _errorMessage!,
                  style: AppTypography.captionSans(color: colors.errorText),
                ),
              ],
              const SizedBox(height: AppTokens.space12),
              TextField(
                key: const Key('plugin_asset_search_field'),
                controller: _searchController,
                onChanged: _onSearchChanged,
                style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                decoration: InputDecoration(
                  hintText: 'Search icons or assets...',
                  isDense: true,
                  prefixIcon:
                      Icon(Icons.search, size: 18, color: colors.mutedInk),
                  suffixIcon: _searchQuery.isNotEmpty
                      ? IconButton(
                          icon: Icon(
                            Icons.clear,
                            size: 16,
                            color: colors.mutedInk,
                          ),
                          onPressed: () {
                            _searchController.clear();
                            _onSearchChanged('');
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
                child: _isLoading
                    ? Center(
                        child: CircularProgressIndicator(
                          color: colors.accentPrimary,
                        ),
                      )
                    : filtered.isEmpty
                        ? Center(
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
                                  widget.field.emptyMessage ??
                                      'No matching assets found.',
                                  textAlign: TextAlign.center,
                                  style: AppTypography.bodySans(
                                    fontSize: 13,
                                    color: colors.secondaryInk,
                                  ),
                                ),
                              ],
                            ),
                          )
                        : GridView.builder(
                            gridDelegate:
                                const SliverGridDelegateWithMaxCrossAxisExtent(
                              maxCrossAxisExtent: 96,
                              mainAxisSpacing: AppTokens.space8,
                              crossAxisSpacing: AppTokens.space8,
                              childAspectRatio: 0.85,
                            ),
                            itemCount: filtered.length,
                            itemBuilder: (context, index) {
                              final item = filtered[index];
                              final isSelected = widget.currentValue != null &&
                                  (widget.currentValue == item.value ||
                                      widget.currentValue == item.iconUri);

                              return InkWell(
                                key: Key('plugin_asset_tile_${item.value}'),
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                onTap: () => Navigator.of(context).pop(item),
                                child: Container(
                                  padding:
                                      const EdgeInsets.all(AppTokens.space8),
                                  decoration: BoxDecoration(
                                    color: isSelected
                                        ? colors.accentContainer
                                        : colors.containerLow,
                                    borderRadius: BorderRadius.circular(
                                      AppTokens.radiusSm,
                                    ),
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
                                          color: _isSquarePixel
                                              ? AppTokens.obsidianBackground
                                              : colors.container,
                                          borderRadius:
                                              BorderRadius.circular(4),
                                        ),
                                        alignment: Alignment.center,
                                        child: (item.imageUrl != null &&
                                                item.imageUrl!.isNotEmpty)
                                            ? Image.network(
                                                item.imageUrl!,
                                                width: 28,
                                                height: 28,
                                                fit: BoxFit.contain,
                                                filterQuality: _isSquarePixel
                                                    ? FilterQuality.none
                                                    : FilterQuality.low,
                                                errorBuilder: (_, _, _) =>
                                                    const Icon(
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
                                        item.label,
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
                          ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
