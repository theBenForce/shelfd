import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/plugin.dart';
import '../../../data/services/api_service.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import '../settings/plugin_config_dialog.dart';
import 'plugin_asset_picker_dialog.dart';

Future<PluginActionResult?> showPluginActionDialog(
  BuildContext context, {
  required PluginSummary plugin,
  required PluginActionDef action,
  required PluginActionTarget target,
}) {
  return showDialog<PluginActionResult>(
    context: context,
    barrierDismissible: false,
    builder: (ctx) => PluginActionDialog(
      plugin: plugin,
      action: action,
      target: target,
    ),
  );
}

class PluginActionDialog extends ConsumerStatefulWidget {
  final PluginSummary plugin;
  final PluginActionDef action;
  final PluginActionTarget target;

  const PluginActionDialog({
    super.key,
    required this.plugin,
    required this.action,
    required this.target,
  });

  @override
  ConsumerState<PluginActionDialog> createState() => _PluginActionDialogState();
}

class _PluginActionDialogState extends ConsumerState<PluginActionDialog> {
  PluginActionFormSpec? _form;
  bool _isPreparing = true;
  bool _isSubmitting = false;
  String? _errorMessage;

  final Map<String, dynamic> _values = {};
  final Map<String, TextEditingController> _textControllers = {};
  final Map<String, TextEditingController> _remoteSearchControllers = {};
  final Map<String, Timer> _remoteSearchDebouncers = {};
  final Map<String, List<PluginOptionItem>> _remoteOptions = {};
  final Map<String, bool> _remoteLoading = {};
  final Map<String, String> _remoteError = {};
  final Map<String, PluginOptionItem> _selectedAssetItems = {};
  final Map<String, Map<int, PluginOptionItem>> _entryOverrideAssetItems = {};

  @override
  void initState() {
    super.initState();
    _loadForm();
  }

  @override
  void dispose() {
    for (final c in _textControllers.values) {
      c.dispose();
    }
    for (final c in _remoteSearchControllers.values) {
      c.dispose();
    }
    for (final t in _remoteSearchDebouncers.values) {
      t.cancel();
    }
    super.dispose();
  }

  Future<void> _loadForm() async {
    setState(() {
      _isPreparing = true;
      _errorMessage = null;
    });

    try {
      final api = ref.read(apiServiceProvider);
      final form = await api.preparePluginAction(
        widget.plugin.name,
        widget.action.name,
        target: widget.target,
      );
      if (!mounted) return;

      _initializeValuesFromForm(form);
      setState(() {
        _form = form;
        _isPreparing = false;
      });

      if (!form.authRequired) {
        for (final field in form.fields) {
          if (field.kind == 'remote_select' &&
              field.optionsSource != null &&
              field.optionsSource!.isNotEmpty) {
            unawaited(_loadRemoteOptions(field));
          }
        }
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isPreparing = false;
        _errorMessage = 'Failed to prepare action: $e';
      });
    }
  }

  void _initializeValuesFromForm(PluginActionFormSpec form) {
    for (final field in form.fields) {
      if (field.kind == 'text') {
        final initialText = field.defaultValue?.toString() ?? '';
        _values[field.key] = initialText;
        if (_textControllers.containsKey(field.key)) {
          _textControllers[field.key]!.text = initialText;
        } else {
          final ctrl = TextEditingController(text: initialText);
          ctrl.addListener(() {
            _values[field.key] = ctrl.text;
          });
          _textControllers[field.key] = ctrl;
        }
      } else if (field.kind == 'boolean') {
        _values[field.key] =
            field.defaultValue is bool ? field.defaultValue as bool : false;
      } else if (field.kind == 'segmented' || field.kind == 'select') {
        final def = field.defaultValue?.toString();
        if (def != null && def.isNotEmpty) {
          _values[field.key] = def;
        } else if (field.options.isNotEmpty) {
          _values[field.key] = field.options.first.value;
        } else {
          _values[field.key] = '';
        }
      } else if (field.kind == 'remote_select') {
        _values[field.key] = field.defaultValue?.toString() ?? '';
        _remoteOptions[field.key] = field.options;
        _remoteSearchControllers.putIfAbsent(
          field.key,
          () => TextEditingController(),
        );
      } else if (field.kind == 'asset_picker') {
        final def = field.defaultValue?.toString() ?? '';
        _values[field.key] = def;
        final ovKey = field.overrideKey ?? '${field.key}_overrides';
        _values[ovKey] = <String, String>{};
        _entryOverrideAssetItems[field.key] = <int, PluginOptionItem>{};
      } else {
        _values[field.key] = field.defaultValue;
      }
    }
  }

  Future<void> _loadRemoteOptions(
    PluginInputFieldSpec field, {
    String? query,
  }) async {
    final source = field.optionsSource;
    if (source == null || source.isEmpty) return;

    setState(() {
      _remoteLoading[field.key] = true;
      _remoteError.remove(field.key);
    });

    try {
      final api = ref.read(apiServiceProvider);
      final res = await api.getPluginActionOptions(
        widget.plugin.name,
        widget.action.name,
        source: source,
        query: (query != null && query.trim().isNotEmpty) ? query.trim() : null,
        targetType: widget.target.type,
        targetId: widget.target.id,
      );
      if (!mounted) return;
      setState(() {
        _remoteOptions[field.key] = res.items;
        _remoteLoading[field.key] = false;
        final currentVal = _values[field.key]?.toString() ?? '';
        if (currentVal.isEmpty && res.items.isNotEmpty) {
          _values[field.key] = res.items.first.value;
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _remoteLoading[field.key] = false;
        _remoteError[field.key] = 'Failed to load options: $e';
      });
    }
  }

  Future<void> _handleOpenPluginConfig() async {
    await showDialog<bool>(
      context: context,
      builder: (_) => PluginConfigDialog(
        plugin: widget.plugin,
      ),
    );
    if (mounted) {
      await _loadForm();
    }
  }

  Future<void> _handlePickDefaultAsset(PluginInputFieldSpec field) async {
    final picked = await showPluginAssetPickerDialog(
      context,
      pluginName: widget.plugin.name,
      actionName: widget.action.name,
      field: field,
      currentValue: _values[field.key]?.toString(),
      targetLabel: 'Default asset for "${widget.target.title ?? widget.action.title}"',
    );
    if (picked != null && mounted) {
      setState(() {
        _selectedAssetItems[field.key] = picked;
        _values[field.key] = picked.value;
      });
    }
  }

  Future<void> _handlePickEntryAsset(
    PluginInputFieldSpec field,
    PluginFormEntry entry,
  ) async {
    final ovKey = field.overrideKey ?? '${field.key}_overrides';
    final ovMap = (_values[ovKey] is Map<String, String>)
        ? (_values[ovKey] as Map<String, String>)
        : <String, String>{};
    final currentVal =
        ovMap['${entry.index}'] ?? _values[field.key]?.toString();

    final picked = await showPluginAssetPickerDialog(
      context,
      pluginName: widget.plugin.name,
      actionName: widget.action.name,
      field: field,
      currentValue: currentVal,
      targetLabel: '${entry.index + 1}. ${entry.title}',
    );
    if (picked != null && mounted) {
      setState(() {
        final nextMap = Map<String, String>.from(ovMap);
        nextMap['${entry.index}'] = picked.value;
        _values[ovKey] = nextMap;
        _entryOverrideAssetItems
            .putIfAbsent(field.key, () => {})[entry.index] = picked;
      });
    }
  }

  Future<void> _handleSubmit() async {
    final form = _form;
    if (form == null) return;

    setState(() {
      _errorMessage = null;
    });

    for (final field in form.fields) {
      if (!field.isVisible(_values)) continue;
      if (field.required) {
        final v = _values[field.key];
        if (v == null || (v is String && v.trim().isEmpty)) {
          setState(() {
            _errorMessage = '${field.title} is required.';
          });
          return;
        }
      }
      if (field.kind == 'remote_select') {
        final v = _values[field.key]?.toString() ?? '';
        if (v.trim().isEmpty) {
          setState(() {
            _errorMessage = 'Please select an option for ${field.title}.';
          });
          return;
        }
      }
    }

    setState(() {
      _isSubmitting = true;
    });

    try {
      final payloadValues = <String, dynamic>{};
      final overrides = <String, String>{};

      for (final field in form.fields) {
        if (field.kind == 'text') {
          payloadValues[field.key] =
              _textControllers[field.key]?.text.trim() ??
                  _values[field.key]?.toString() ??
                  '';
        } else {
          payloadValues[field.key] = _values[field.key];
        }
        if (field.perEntryOverride) {
          final ovKey = field.overrideKey ?? '${field.key}_overrides';
          final ovVal = _values[ovKey];
          if (ovVal is Map<String, String> && ovVal.isNotEmpty) {
            payloadValues[ovKey] = ovVal;
            overrides.addAll(ovVal);
          }
        }
      }
      if (widget.target.bookIds != null && widget.target.bookIds!.isNotEmpty) {
        payloadValues['book_ids'] = widget.target.bookIds;
      }

      final api = ref.read(apiServiceProvider);
      final result = await api.invokePluginAction(
        widget.plugin.name,
        widget.action.name,
        target: widget.target,
        input: payloadValues,
        overrides: overrides.isNotEmpty ? overrides : null,
      );

      if (result.job != null) {
        ref.read(pluginJobsProvider.notifier).enqueueOrTrackJob(result.job!);
      } else if (result.isJobStarted && result.jobId != null) {
        ref.read(pluginJobsProvider.notifier).enqueueOrTrackJob(
              PluginJob(
                id: result.jobId!,
                plugin: widget.plugin.name,
                action: widget.action.name,
                title: widget.target.title ?? widget.action.title,
                targetType: widget.target.type,
                targetId: widget.target.id,
                status: 'running',
                message: result.message ?? 'Starting...',
              ),
            );
      }

      if (mounted) {
        Navigator.of(context).pop(result);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
          _errorMessage = e is ApiException ? e.message : e.toString();
        });
      }
    }
  }

  IconData _resolveActionIcon(String? iconName) {
    switch (iconName) {
      case 'speaker':
        return Icons.speaker_rounded;
      case 'auto_awesome':
        return Icons.auto_awesome_rounded;
      default:
        return Icons.extension_rounded;
    }
  }

  Widget _buildAssetPreviewBadge(
    ShelfdColors colors,
    PluginOptionItem? item, {
    required bool isSquarePixel,
  }) {
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(
        color: isSquarePixel ? AppTokens.obsidianBackground : colors.container,
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: colors.subtleBorder),
      ),
      alignment: Alignment.center,
      child: (item != null && item.imageUrl != null && item.imageUrl!.isNotEmpty)
          ? Image.network(
              item.imageUrl!,
              width: 22,
              height: 22,
              fit: BoxFit.contain,
              filterQuality:
                  isSquarePixel ? FilterQuality.none : FilterQuality.low,
              errorBuilder: (_, _, _) => const Icon(
                Icons.auto_stories,
                size: 16,
                color: AppTokens.amberAccent,
              ),
            )
          : const Icon(
              Icons.auto_stories,
              size: 16,
              color: AppTokens.amberAccent,
            ),
    );
  }

  Widget _buildFieldWidget(
    BuildContext context,
    ShelfdColors colors,
    PluginActionFormSpec form,
    PluginInputFieldSpec field,
  ) {
    switch (field.kind) {
      case 'segmented':
        final options = field.options;
        if (options.isEmpty) return const SizedBox.shrink();
        final currentVal = _values[field.key]?.toString() ?? options.first.value;
        final validSelected = options.any((o) => o.value == currentVal)
            ? currentVal
            : options.first.value;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              field.title,
              style: AppTypography.bodySans(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            if (field.description != null &&
                field.description!.trim().isNotEmpty) ...[
              const SizedBox(height: AppTokens.space4),
              Text(
                field.description!,
                style: AppTypography.captionSans(color: colors.mutedInk),
              ),
            ],
            const SizedBox(height: AppTokens.space8),
            SizedBox(
              width: double.infinity,
              child: SegmentedButton<String>(
                key: Key('plugin_action_field_${field.key}'),
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
                segments: options
                    .map(
                      (o) => ButtonSegment<String>(
                        value: o.value,
                        label: Text(o.label),
                      ),
                    )
                    .toList(),
                selected: {validSelected},
                onSelectionChanged: _isSubmitting
                    ? null
                    : (selection) {
                        if (selection.isNotEmpty) {
                          setState(() {
                            _values[field.key] = selection.first;
                            _errorMessage = null;
                          });
                        }
                      },
              ),
            ),
          ],
        );

      case 'select':
        final options = field.options;
        if (options.isEmpty) return const SizedBox.shrink();
        final currentVal = _values[field.key]?.toString() ?? options.first.value;
        final validSelected = options.any((o) => o.value == currentVal)
            ? currentVal
            : options.first.value;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              field.title,
              style: AppTypography.bodySans(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            DropdownButtonFormField<String>(
              key: Key('plugin_action_field_${field.key}'),
              initialValue: validSelected,
              dropdownColor: colors.surface,
              style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
              decoration: InputDecoration(
                isDense: true,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  borderSide: BorderSide(color: colors.border),
                ),
              ),
              items: options
                  .map(
                    (o) => DropdownMenuItem<String>(
                      value: o.value,
                      child: Text(o.label),
                    ),
                  )
                  .toList(),
              onChanged: _isSubmitting
                  ? null
                  : (val) {
                      if (val != null) {
                        setState(() {
                          _values[field.key] = val;
                        });
                      }
                    },
            ),
          ],
        );

      case 'boolean':
        final val = _values[field.key] == true;
        return Container(
          decoration: BoxDecoration(
            color: colors.containerLow,
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: colors.subtleBorder),
          ),
          child: Material(
            type: MaterialType.transparency,
            child: SwitchListTile.adaptive(
              key: Key('plugin_action_field_${field.key}'),
              dense: true,
              title: Text(
                field.title,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
              subtitle: (field.description != null &&
                      field.description!.trim().isNotEmpty)
                  ? Text(
                      field.description!,
                      style: AppTypography.captionSans(color: colors.mutedInk),
                    )
                  : null,
              value: val,
              activeTrackColor: colors.accentPrimary,
              onChanged: _isSubmitting
                  ? null
                  : (next) {
                      setState(() {
                        _values[field.key] = next;
                      });
                    },
            ),
          ),
        );

      case 'remote_select':
        final options = _remoteOptions[field.key] ?? const [];
        final isLoading = _remoteLoading[field.key] == true;
        final err = _remoteError[field.key];
        final searchCtrl = _remoteSearchControllers[field.key];
        final q = searchCtrl?.text.trim().toLowerCase() ?? '';
        final filtered = q.isEmpty
            ? options
            : options.where((item) {
                return item.label.toLowerCase().contains(q) ||
                    item.value.toLowerCase().contains(q) ||
                    (item.subtitle?.toLowerCase().contains(q) ?? false);
              }).toList();
        final selectedValue = _values[field.key]?.toString() ?? '';

        return Column(
          key: Key('plugin_action_field_${field.key}'),
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              field.title,
              style: AppTypography.bodySans(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            if (field.searchable) ...[
              TextField(
                key: Key('plugin_action_search_${field.key}'),
                controller: searchCtrl,
                onChanged: (val) {
                  setState(() {});
                  _remoteSearchDebouncers[field.key]?.cancel();
                  _remoteSearchDebouncers[field.key] = Timer(
                    const Duration(milliseconds: 220),
                    () {
                      if (mounted) {
                        _loadRemoteOptions(field, query: val);
                      }
                    },
                  );
                },
                style: AppTypography.bodySans(fontSize: 13, color: colors.ink),
                decoration: InputDecoration(
                  hintText:
                      field.placeholder ?? 'Search ${field.title.toLowerCase()}...',
                  isDense: true,
                  prefixIcon:
                      Icon(Icons.search, size: 18, color: colors.mutedInk),
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
              const SizedBox(height: AppTokens.space8),
            ],
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 160),
              child: isLoading
                  ? Padding(
                      padding: const EdgeInsets.all(AppTokens.space16),
                      child: Center(
                        child: CircularProgressIndicator(
                          color: colors.accentPrimary,
                        ),
                      ),
                    )
                  : err != null
                      ? Padding(
                          padding: const EdgeInsets.all(AppTokens.space12),
                          child: Text(
                            err,
                            style: AppTypography.bodySans(
                              fontSize: 13,
                              color: colors.errorText,
                            ),
                          ),
                        )
                      : filtered.isEmpty
                          ? Container(
                              width: double.infinity,
                              padding: const EdgeInsets.all(AppTokens.space16),
                              decoration: BoxDecoration(
                                color: colors.containerLow,
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                border: Border.all(color: colors.subtleBorder),
                              ),
                              child: Text(
                                field.emptyMessage ?? 'No options found.',
                                style: AppTypography.bodySans(
                                  fontSize: 13,
                                  color: colors.secondaryInk,
                                ),
                              ),
                            )
                          : ListView.separated(
                              shrinkWrap: true,
                              itemCount: filtered.length,
                              separatorBuilder: (_, _) =>
                                  const SizedBox(height: AppTokens.space4),
                              itemBuilder: (context, idx) {
                                final item = filtered[idx];
                                final isSelected = selectedValue == item.value;
                                return InkWell(
                                  key: Key(
                                    'plugin_action_option_${field.key}_${item.value}',
                                  ),
                                  borderRadius:
                                      BorderRadius.circular(AppTokens.radiusSm),
                                  onTap: _isSubmitting
                                      ? null
                                      : () {
                                          setState(() {
                                            _values[field.key] = item.value;
                                          });
                                        },
                                  child: Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: AppTokens.space12,
                                      vertical: AppTokens.space8,
                                    ),
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
                                      ),
                                    ),
                                    child: Row(
                                      children: [
                                        Icon(
                                          isSelected
                                              ? Icons.radio_button_checked
                                              : Icons.radio_button_off,
                                          size: 18,
                                          color: isSelected
                                              ? colors.accentPrimary
                                              : colors.mutedInk,
                                        ),
                                        const SizedBox(width: AppTokens.space12),
                                        Expanded(
                                          child: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.start,
                                            children: [
                                              Text(
                                                item.label,
                                                maxLines: 1,
                                                overflow: TextOverflow.ellipsis,
                                                style: AppTypography.bodySans(
                                                  fontSize: 14,
                                                  fontWeight: isSelected
                                                      ? FontWeight.w600
                                                      : FontWeight.w500,
                                                  color: colors.ink,
                                                ),
                                              ),
                                              if (item.subtitle != null &&
                                                  item.subtitle!.isNotEmpty)
                                                Text(
                                                  item.subtitle!,
                                                  maxLines: 1,
                                                  overflow:
                                                      TextOverflow.ellipsis,
                                                  style: AppTypography
                                                      .captionSans(
                                                    fontSize: 11,
                                                    color: colors.secondaryInk,
                                                  ),
                                                ),
                                            ],
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
        );

      case 'asset_picker':
        final isSquarePixel = field.assetShape == 'square_pixel';
        final selectedItem = _selectedAssetItems[field.key];
        final currentVal = _values[field.key]?.toString() ?? '';
        final ovKey = field.overrideKey ?? '${field.key}_overrides';
        final ovMap = (_values[ovKey] is Map<String, String>)
            ? (_values[ovKey] as Map<String, String>)
            : const <String, String>{};
        final ovItems = _entryOverrideAssetItems[field.key] ?? const {};

        return Column(
          key: Key('plugin_action_field_${field.key}'),
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Container(
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: colors.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: colors.subtleBorder),
              ),
              child: Row(
                children: [
                  _buildAssetPreviewBadge(
                    colors,
                    selectedItem,
                    isSquarePixel: isSquarePixel,
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          field.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        Text(
                          selectedItem != null
                              ? selectedItem.label
                              : (currentVal.isNotEmpty
                                  ? 'Default Icon'
                                  : 'None selected'),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.captionSans(
                            fontSize: 11,
                            color: colors.secondaryInk,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  OutlinedButton.icon(
                    key: Key('plugin_action_pick_asset_${field.key}'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.ink,
                      side: BorderSide(color: colors.border),
                    ),
                    onPressed: _isSubmitting
                        ? null
                        : () => _handlePickDefaultAsset(field),
                    icon: const Icon(Icons.grid_view_rounded, size: 15),
                    label: const Text('Choose Icon'),
                  ),
                ],
              ),
            ),
            if (field.perEntryOverride && form.entries.isNotEmpty) ...[
              const SizedBox(height: AppTokens.space8),
              Container(
                decoration: BoxDecoration(
                  color: colors.containerLow,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  border: Border.all(color: colors.subtleBorder),
                ),
                child: Material(
                  type: MaterialType.transparency,
                  child: ExpansionTile(
                    key: Key('plugin_action_overrides_tile_${field.key}'),
                    tilePadding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space12,
                    ),
                    title: Text(
                      field.overrideLabel ??
                          'Customize Per-Item (${form.entries.length})',
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: colors.ink,
                      ),
                    ),
                    subtitle: ovMap.isNotEmpty
                        ? Text(
                            '${ovMap.length} custom override(s) selected',
                            style: AppTypography.captionSans(
                              fontSize: 11,
                              color: colors.accentPrimary,
                            ),
                          )
                        : null,
                    children: [
                      ConstrainedBox(
                        constraints: const BoxConstraints(maxHeight: 200),
                        child: ListView.separated(
                          shrinkWrap: true,
                          padding: const EdgeInsets.fromLTRB(
                            AppTokens.space12,
                            0,
                            AppTokens.space12,
                            AppTokens.space12,
                          ),
                          itemCount: form.entries.length,
                          separatorBuilder: (_, _) =>
                              const SizedBox(height: AppTokens.space4),
                          itemBuilder: (context, idx) {
                            final entry = form.entries[idx];
                            final customItem = ovItems[entry.index];
                            final hasOverride =
                                ovMap.containsKey('${entry.index}');
                            return Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppTokens.space8,
                                vertical: AppTokens.space4,
                              ),
                              decoration: BoxDecoration(
                                color: colors.surface,
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                border: Border.all(color: colors.subtleBorder),
                              ),
                              child: Row(
                                children: [
                                  _buildAssetPreviewBadge(
                                    colors,
                                    customItem ?? selectedItem,
                                    isSquarePixel: isSquarePixel,
                                  ),
                                  const SizedBox(width: AppTokens.space8),
                                  Expanded(
                                    child: Text(
                                      '${entry.index + 1}. ${entry.title}',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: AppTypography.bodySans(
                                        fontSize: 12,
                                        fontWeight: hasOverride
                                            ? FontWeight.w600
                                            : FontWeight.w400,
                                        color: colors.ink,
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: AppTokens.space8),
                                  TextButton(
                                    key: Key(
                                      'plugin_action_entry_asset_${field.key}_${entry.index}',
                                    ),
                                    onPressed: _isSubmitting
                                        ? null
                                        : () =>
                                            _handlePickEntryAsset(field, entry),
                                    child: Text(
                                      hasOverride ? 'Change' : 'Pick Icon',
                                      style: AppTypography.captionSans(
                                        color: colors.accentPrimary,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            );
                          },
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ],
        );

      case 'text':
      default:
        final ctrl = _textControllers.putIfAbsent(
          field.key,
          () => TextEditingController(
            text: _values[field.key]?.toString() ?? '',
          ),
        );
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              field.required ? '${field.title} *' : field.title,
              style: AppTypography.bodySans(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
            ),
            if (field.description != null &&
                field.description!.trim().isNotEmpty) ...[
              const SizedBox(height: AppTokens.space4),
              Text(
                field.description!,
                style: AppTypography.captionSans(color: colors.mutedInk),
              ),
            ],
            const SizedBox(height: AppTokens.space8),
            TextFormField(
              key: Key('plugin_action_field_${field.key}'),
              controller: ctrl,
              enabled: !_isSubmitting,
              style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
              onChanged: (v) {
                _values[field.key] = v;
              },
              decoration: InputDecoration(
                hintText: field.placeholder,
                isDense: true,
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
          ],
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final form = _form;
    final headerTitle = form?.title ?? widget.action.title;
    final entriesCount = form?.entries.length ?? 0;
    final headerSubtitle = widget.target.title != null &&
            widget.target.title!.isNotEmpty
        ? (entriesCount > 0
            ? '${widget.target.title} • $entriesCount ${entriesCount == 1 ? 'item' : 'items'}'
            : widget.target.title!)
        : widget.plugin.displayName;

    return Dialog(
      key: Key('plugin_action_dialog_${widget.plugin.name}_${widget.action.name}'),
      backgroundColor: colors.surface,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: BorderSide(color: colors.border),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: 580,
          maxHeight: 720,
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(AppTokens.space8),
                    decoration: BoxDecoration(
                      color: colors.accentContainer,
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    ),
                    child: Icon(
                      _resolveActionIcon(widget.action.icon),
                      size: 20,
                      color: colors.accentPrimary,
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          headerTitle,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: 20,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        Text(
                          headerSubtitle,
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
                    onPressed: _isSubmitting
                        ? null
                        : () => Navigator.of(context).pop(),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Divider(height: 1, color: colors.subtleBorder),
              const SizedBox(height: AppTokens.space16),
              if (_isPreparing)
                Padding(
                  padding:
                      const EdgeInsets.symmetric(vertical: AppTokens.space32),
                  child: Center(
                    child: CircularProgressIndicator(
                      color: colors.accentPrimary,
                    ),
                  ),
                )
              else if (form != null && form.authRequired)
                Padding(
                  padding:
                      const EdgeInsets.symmetric(vertical: AppTokens.space24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.link_off_rounded,
                        size: 44,
                        color: colors.mutedInk,
                      ),
                      const SizedBox(height: AppTokens.space12),
                      Text(
                        '${widget.plugin.displayName} Account Not Linked',
                        style: AppTypography.titleSerif(
                          fontSize: 18,
                          color: colors.ink,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        form.authMessage ??
                            'Configure and link your ${widget.plugin.displayName} account before running this action.',
                        textAlign: TextAlign.center,
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          color: colors.secondaryInk,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space20),
                      FilledButton.icon(
                        key: const Key('plugin_action_configure_auth_button'),
                        style: FilledButton.styleFrom(
                          backgroundColor: colors.accentPrimary,
                          foregroundColor: colors.onAccent,
                        ),
                        onPressed: _handleOpenPluginConfig,
                        icon: const Icon(Icons.tune_rounded, size: 18),
                        label: Text('Configure ${widget.plugin.displayName}'),
                      ),
                    ],
                  ),
                )
              else if (form != null) ...[
                if (_errorMessage != null) ...[
                  Container(
                    padding: const EdgeInsets.all(AppTokens.space12),
                    margin: const EdgeInsets.only(bottom: AppTokens.space12),
                    decoration: BoxDecoration(
                      color: colors.errorBg,
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                      border: Border.all(color: colors.errorBorder),
                    ),
                    child: Text(
                      _errorMessage!,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        color: colors.errorText,
                      ),
                    ),
                  ),
                ],
                Flexible(
                  child: SingleChildScrollView(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        ...form.fields
                            .where((f) => f.isVisible(_values))
                            .map((field) {
                          return Padding(
                            padding: const EdgeInsets.only(
                              bottom: AppTokens.space16,
                            ),
                            child: _buildFieldWidget(
                              context,
                              colors,
                              form,
                              field,
                            ),
                          );
                        }),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: AppTokens.space12),
                Divider(height: 1, color: colors.subtleBorder),
                const SizedBox(height: AppTokens.space16),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton(
                      onPressed: _isSubmitting
                          ? null
                          : () => Navigator.of(context).pop(),
                      child: Text(
                        'Cancel',
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          color: colors.secondaryInk,
                        ),
                      ),
                    ),
                    const SizedBox(width: AppTokens.space12),
                    FilledButton.icon(
                      key: const Key('plugin_action_submit_button'),
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.accentPrimary,
                        foregroundColor: colors.onAccent,
                      ),
                      onPressed: _isSubmitting ? null : _handleSubmit,
                      icon: _isSubmitting
                          ? SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: colors.onAccent,
                              ),
                            )
                          : Icon(
                              _resolveActionIcon(widget.action.icon),
                              size: 16,
                            ),
                      label: Text(
                        _isSubmitting
                            ? 'Running...'
                            : (form.submitLabel ?? widget.action.title),
                      ),
                    ),
                  ],
                ),
              ] else if (_errorMessage != null) ...[
                Padding(
                  padding: const EdgeInsets.all(AppTokens.space16),
                  child: Text(
                    _errorMessage!,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.errorText,
                    ),
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
