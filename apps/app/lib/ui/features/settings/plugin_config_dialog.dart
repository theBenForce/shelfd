import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../data/models/plugin.dart';
import '../../core/file_saver/file_saver.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class PluginConfigDialog extends ConsumerStatefulWidget {
  final PluginSummary plugin;
  final VoidCallback? onSaved;

  const PluginConfigDialog({
    super.key,
    required this.plugin,
    this.onSaved,
  });

  @override
  ConsumerState<PluginConfigDialog> createState() => _PluginConfigDialogState();
}

class _PluginConfigDialogState extends ConsumerState<PluginConfigDialog>
    with WidgetsBindingObserver {
  final Map<String, TextEditingController> _controllers = {};
  final Map<String, TextEditingController> _authControllers = {};
  final TextEditingController _authCodeController = TextEditingController();
  final Map<String, bool> _boolValues = {};
  final Map<String, String> _enumValues = {};
  final Set<String> _obscuredKeys = {};
  final Set<String> _clearedSecrets = {};
  Set<String> _secretFieldsSet = {};

  // Repeatable object-array support.
  final Map<String, List<Map<String, dynamic>>> _arrayItems = {};
  final Map<String, TextEditingController> _itemControllers = {};
  final Set<String> _setItemSecretsLoaded = {};

  PluginAuthStatus? _authStatus;
  bool _isAuthBusy = false;
  bool _showTokenAuthFields = false;
  String? _authSuccessMessage;

  bool _isLoading = true;
  bool _isSaving = false;
  String? _runningCommand;
  String? _errorMessage;
  String? _commandResultMessage;

  Map<String, dynamic> get _properties {
    final raw = widget.plugin.configSchema?['properties'];
    if (raw is Map) {
      return Map<String, dynamic>.from(raw);
    }
    return const {};
  }

  Set<String> get _requiredFields {
    final raw = widget.plugin.configSchema?['required'];
    if (raw is List) {
      return raw.map((e) => e.toString()).toSet();
    }
    return const {};
  }

  bool _isSecretProp(Map<String, dynamic> prop) {
    return prop['x-shelfd-secret'] == true || prop['format'] == 'password';
  }

  bool _isHiddenProp(Map<String, dynamic> prop) {
    return prop['x-shelfd-hidden'] == true;
  }

  bool _supportsCurlPaste(Map<String, dynamic> prop) {
    return prop['x-shelfd-curl-paste'] == true;
  }

  /// Returns true when [prop] declares an array whose items are objects.
  bool _isArrayObjectCard(Map<String, dynamic> prop) {
    return (prop['type']?.toString() ?? '') == 'array' &&
        (prop['items'] as Map?)?['type']?.toString() == 'object';
  }

  /// Extracts the nested properties from an array-item object schema.
  Map<String, dynamic> _itemProperties(Map<String, dynamic> prop) {
    final items = prop['items'];
    if (items is Map && items['properties'] is Map) {
      return Map<String, dynamic>.from(items['properties'] as Map);
    }
    return {};
  }

  /// Adds a new blank item to [key]'s array and registers its field controllers.
  void _addArrayItem(String key) {
    setState(() {
      final items = _arrayItems.putIfAbsent(key, () => []);
      final idx = items.length;
      items.add({});
      final itemProps = _itemProperties(_visibleProperties[key]!);
      for (final entry in itemProps.entries) {
        final iKey = '$key.$idx.${entry.key}';
        if (_itemControllers[iKey] == null) {
          final val = entry.value['default']?.toString();
          _itemControllers[iKey] = TextEditingController(text: val ?? '');
        }
      }
    });
  }

  /// Removes the item at [index] from [key]'s array and cleans up controllers.
  void _removeArrayItem(String key, int index) {
    setState(() {
      final items = _arrayItems[key];
      if (items == null || index >= items.length) return;
      items.removeAt(index);
      // Clean up controllers for the removed item.
      _itemControllers.removeWhere((k, _) => k.startsWith('$key.$index.'));
      // Re-index remaining controllers to fill the gap.
      final remaining = _arrayItems[key] ?? [];
      for (var rIdx = 0; rIdx < remaining.length; rIdx++) {
        final oldPrefix = '$key.${rIdx + 1}.';
        final newPrefix = '$key.$rIdx.';
        for (final oldKey in _itemControllers.keys.toList()) {
          if (oldKey.startsWith(oldPrefix)) {
            final newKey = newPrefix + oldKey.substring(oldPrefix.length);
            _itemControllers[newKey] = _itemControllers.remove(oldKey)!;
          }
        }
      }
    });
  }

  /// Serializes [key]'s items into a JSON-ready list; blank cards are dropped,
  /// and per-item secrets send the entered value or the redaction sentinel only
  /// when a stored value must be preserved.
  List<dynamic> _serializeArrayItems(String key) {
    final itemProps = _itemProperties(_visibleProperties[key]!);
    final items = _arrayItems[key] ?? [];
    final result = <dynamic>[];
    for (var idx = 0; idx < items.length; idx++) {
      final item = items[idx];
      final itemMap = <String, dynamic>{};
      for (final propEntry in itemProps.entries) {
        final fieldKey = propEntry.key;
        final ctrl = _itemControllers['$key.$idx.$fieldKey'];
        final rawText = ctrl?.text.trim() ?? '';
        final isItemSecret = _isSecretProp((propEntry.value as Map?)?.cast<String, dynamic>() ?? {});
        if (isItemSecret && rawText.isNotEmpty) {
          itemMap[fieldKey] = rawText;
        } else if (isItemSecret && _setItemSecretsLoaded.contains('$key.items[$idx].$fieldKey')) {
          itemMap[fieldKey] = kPluginSecretRedactedSentinel;
        } else if (rawText.isNotEmpty || item.containsKey(fieldKey)) {
          itemMap[fieldKey] = rawText;
        }
      }
      if (itemMap.isEmpty) continue; // blank fresh card: don't persist it
      result.add(itemMap);
    }
    return result;
  }

  /// Loads array item initial values from saved config.
  void _loadArrayConfig(String key, Map<String, dynamic>? configData) {
    if (configData == null || configData[key] == null) return;
    final arr = configData[key];
    if (arr is! List) return;
    setState(() {
      final items = arr.map((e) => e as Map<String, dynamic>).toList();
      _arrayItems[key] = items;
      final itemProps = _itemProperties(_visibleProperties[key]!);
      for (var idx = 0; idx < items.length; idx++) {
        for (final propEntry in itemProps.entries) {
          final fieldKey = propEntry.key;
          final ctrlKey = '$key.$idx.$fieldKey';
          final val = items[idx][fieldKey]?.toString() ?? '';
          final isItemSecret = _isSecretProp((propEntry.value as Map?)?.cast<String, dynamic>() ?? {});
          _itemControllers[ctrlKey] = TextEditingController(
            text: (isItemSecret && val == kPluginSecretRedactedSentinel) ? '' : val,
          );
          // Track which item secrets are already loaded.
          if (isItemSecret && items[idx].containsKey(fieldKey)) {
            _setItemSecretsLoaded.add('$key.items[$idx].$fieldKey');
          }
        }
      }
    });
  }

  Map<String, dynamic> get _visibleProperties {
    final visible = <String, dynamic>{};
    for (final entry in _properties.entries) {
      final prop = entry.value is Map
          ? Map<String, dynamic>.from(entry.value as Map)
          : const <String, dynamic>{};
      if (!_isHiddenProp(prop)) {
        visible[entry.key] = entry.value;
      }
    }
    return visible;
  }

  @override
  void initState() {
    super.initState();
    for (final entry in _visibleProperties.entries) {
      final key = entry.key;
      final prop = entry.value is Map
          ? Map<String, dynamic>.from(entry.value as Map)
          : const <String, dynamic>{};
      final type = prop['type']?.toString() ?? 'string';
      if (type == 'boolean') {
        _boolValues[key] = prop['default'] as bool? ?? false;
      } else if (prop['enum'] is List && (prop['enum'] as List).isNotEmpty) {
        final enumList =
            (prop['enum'] as List).map((e) => e.toString()).toList();
        _enumValues[key] =
            prop['default']?.toString() ?? enumList.first;
      } else {
        final defaultVal = prop['default'];
        _controllers[key] = TextEditingController(
          text: defaultVal != null ? defaultVal.toString() : '',
        );
        if (_isSecretProp(prop)) {
          _obscuredKeys.add(key);
        }
      }
    }

    final authSpec = widget.plugin.auth;
    if (authSpec != null) {
      for (final f in authSpec.fields) {
        _authControllers[f.key] = TextEditingController();
      }
      if (!_authControllers.containsKey('access_token')) {
        _authControllers['access_token'] = TextEditingController();
      }
      if (!_authControllers.containsKey('refresh_token')) {
        _authControllers['refresh_token'] = TextEditingController();
      }
      if (!_authControllers.containsKey('client_id')) {
        _authControllers['client_id'] = TextEditingController();
      }
      if (!authSpec.supportsOAuthPkce && authSpec.supportsToken) {
        _showTokenAuthFields = true;
      }
    }
    WidgetsBinding.instance.addObserver(this);
    _loadExistingConfig();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    for (final c in _controllers.values) {
      c.dispose();
    }
    for (final c in _authControllers.values) {
      c.dispose();
    }
    _authCodeController.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (!kIsWeb &&
        state == AppLifecycleState.resumed &&
        widget.plugin.hasAuth &&
        mounted) {
      _loadExistingConfig();
      ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
    }
  }

  String _resolveServerWebOrigin() {
    final base = Uri.base;
    if (kIsWeb && (base.scheme == 'http' || base.scheme == 'https')) {
      return base.origin;
    }
    final rawBase = ref.read(apiServiceProvider).baseUrl.trim();
    if (rawBase.isNotEmpty) {
      return rawBase.endsWith('/')
          ? rawBase.substring(0, rawBase.length - 1)
          : rawBase;
    }
    if (base.scheme == 'http' || base.scheme == 'https') {
      return base.origin;
    }
    return 'http://localhost:8080';
  }

  String _resolveRedirectUri() {
    final encodedPlugin = Uri.encodeComponent(widget.plugin.name);
    final origin = _resolveServerWebOrigin();
    return '$origin/settings?plugin=$encodedPlugin';
  }

  Future<void> _handleOpenWebDashboardForAuth() async {
    final targetUrl = _resolveRedirectUri();
    await openExternalUrl(targetUrl);
  }

  Future<void> _loadExistingConfig() async {
    try {
      final api = ref.read(apiServiceProvider);
      PluginConfigResponse? res;
      if (widget.plugin.hasConfigSchema) {
        res = await ref
            .read(pluginsProvider.notifier)
            .fetchConfig(widget.plugin.name);
      }
      PluginAuthStatus? authStatus;
      if (widget.plugin.hasAuth) {
        try {
          authStatus = await api.getPluginAuthStatus(widget.plugin.name);
        } catch (e) {
          authStatus = PluginAuthStatus(
            linked: false,
            authError: e.toString(),
          );
        }
      }
      if (!mounted) return;
      setState(() {
        if (res != null) {
          _secretFieldsSet = res.secretFieldsSet.toSet();
          for (final entry in _visibleProperties.entries) {
            final key = entry.key;
            final prop = entry.value is Map
                ? Map<String, dynamic>.from(entry.value as Map)
                : const <String, dynamic>{};
            final val = res.config[key];
            if (val == null) continue;
            final type = prop['type']?.toString() ?? 'string';
            if (type == 'boolean') {
              if (val is bool) _boolValues[key] = val;
            } else if (_enumValues.containsKey(key)) {
              _enumValues[key] = val.toString();
            } else if (_isSecretProp(prop)) {
              if (val.toString() != kPluginSecretRedactedSentinel) {
                _controllers[key]?.text = val.toString();
              } else {
                _controllers[key]?.clear();
              }
            } else {
              _controllers[key]?.text = val.toString();
            }
          }
          for (final entry in _authControllers.entries) {
            final cfgVal = res.config[entry.key];
            if (cfgVal != null &&
                cfgVal.toString().isNotEmpty &&
                cfgVal.toString() != kPluginSecretRedactedSentinel &&
                entry.value.text.isEmpty) {
              entry.value.text = cfgVal.toString();
            }
          }
          // Load array-item state from saved config.
          for (final entry in _visibleProperties.entries) {
            final key = entry.key;
            final prop = entry.value is Map
                ? Map<String, dynamic>.from(entry.value as Map)
                : const <String, dynamic>{};
            if (_isArrayObjectCard(prop)) {
              _loadArrayConfig(key, res.config);
            }
          }
        }
        if (authStatus != null) {
          _authStatus = authStatus;
          if (authStatus.clientId != null &&
              authStatus.clientId!.isNotEmpty &&
              (_authControllers['client_id']?.text.isEmpty ?? false)) {
            _authControllers['client_id']?.text = authStatus.clientId!;
          }
        }
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _errorMessage = 'Failed to load configuration: $e';
      });
    }
  }

  Future<void> _handleBeginOAuth() async {
    setState(() {
      _isAuthBusy = true;
      _errorMessage = null;
      _authSuccessMessage = null;
    });
    try {
      final verifier = generatePkceVerifier();
      final challenge = computePkceS256Challenge(verifier);
      final storage = ref.read(storageServiceProvider);
      await storage.savePluginPkceVerifier(widget.plugin.name, verifier);

      final clientId = _authControllers['client_id']?.text.trim() ??
          _controllers['client_id']?.text.trim() ??
          '';

      final api = ref.read(apiServiceProvider);
      final res = await api.beginPluginAuth(widget.plugin.name, {
        'method': 'oauth_pkce',
        'redirect_uri': _resolveRedirectUri(),
        'code_challenge': challenge,
        'state': widget.plugin.name,
        if (clientId.isNotEmpty) 'client_id': clientId,
      });

      if (res.authorizeUrl.isNotEmpty) {
        await openExternalUrl(res.authorizeUrl);
      }
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _errorMessage = 'Failed to start authorization: $e';
      });
    }
  }

  Future<void> _handleCompleteCodeAuth() async {
    final code = _authCodeController.text.trim();
    if (code.isEmpty) {
      setState(() {
        _errorMessage = 'Please enter the authorization code from the callback URL.';
      });
      return;
    }
    setState(() {
      _isAuthBusy = true;
      _errorMessage = null;
      _authSuccessMessage = null;
    });
    try {
      final storage = ref.read(storageServiceProvider);
      final verifier =
          storage.getPluginPkceVerifier(widget.plugin.name) ?? '';
      final clientId = _authControllers['client_id']?.text.trim() ??
          _controllers['client_id']?.text.trim() ??
          '';
      final api = ref.read(apiServiceProvider);
      final status = await api.completePluginAuth(widget.plugin.name, {
        'method': 'oauth_pkce',
        'code': code,
        if (verifier.isNotEmpty) 'code_verifier': verifier,
        'redirect_uri': _resolveRedirectUri(),
        if (clientId.isNotEmpty) 'client_id': clientId,
      });
      await storage.clearPluginPkceVerifier(widget.plugin.name);
      await ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _authStatus = status;
        _authCodeController.clear();
        _authSuccessMessage = '${widget.plugin.displayName} account linked!';
      });
      widget.onSaved?.call();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _errorMessage = 'Failed to exchange authorization code: $e';
      });
    }
  }

  Future<void> _handleCompleteTokenAuth() async {
    final accessToken = _authControllers['access_token']?.text.trim() ?? '';
    if (accessToken.isEmpty) {
      setState(() {
        _errorMessage = 'Access token is required.';
      });
      return;
    }
    final refreshToken = _authControllers['refresh_token']?.text.trim() ?? '';
    final clientId = _authControllers['client_id']?.text.trim() ??
        _controllers['client_id']?.text.trim() ??
        '';
    final credentials = <String, dynamic>{
      for (final entry in _authControllers.entries)
        if (entry.value.text.trim().isNotEmpty)
          entry.key: entry.value.text.trim(),
    };

    setState(() {
      _isAuthBusy = true;
      _errorMessage = null;
      _authSuccessMessage = null;
    });
    try {
      final api = ref.read(apiServiceProvider);
      final status = await api.completePluginAuth(widget.plugin.name, {
        'method': 'token',
        'access_token': accessToken,
        if (refreshToken.isNotEmpty) 'refresh_token': refreshToken,
        if (clientId.isNotEmpty) 'client_id': clientId,
        'credentials': credentials,
      });
      await ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _authStatus = status;
        _authControllers['access_token']?.clear();
        _authControllers['refresh_token']?.clear();
        _authSuccessMessage = '${widget.plugin.displayName} account linked!';
      });
      widget.onSaved?.call();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _errorMessage = 'Failed to save token: $e';
      });
    }
  }

  Future<void> _handleDisconnectAuth() async {
    setState(() {
      _isAuthBusy = true;
      _errorMessage = null;
      _authSuccessMessage = null;
    });
    try {
      final api = ref.read(apiServiceProvider);
      final status = await api.disconnectPluginAuth(widget.plugin.name);
      await ref
          .read(storageServiceProvider)
          .clearPluginPkceVerifier(widget.plugin.name);
      await ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _authStatus = status;
        _authSuccessMessage = '${widget.plugin.displayName} account unlinked.';
      });
      widget.onSaved?.call();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isAuthBusy = false;
        _errorMessage = 'Failed to disconnect account: $e';
      });
    }
  }

  Future<void> _pasteFromClipboard(String key) async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text?.trim();
    if (text == null || text.isEmpty || !mounted) return;
    setState(() {
      _controllers[key]?.text = text;
      _clearedSecrets.remove(key);
      _obscuredKeys.remove(key);
    });
  }

  Future<void> _handleSave() async {
    setState(() {
      _isSaving = true;
      _errorMessage = null;
      _commandResultMessage = null;
    });

    final payload = <String, dynamic>{};
    for (final entry in _visibleProperties.entries) {
      final key = entry.key;
      final prop = entry.value is Map
          ? Map<String, dynamic>.from(entry.value as Map)
          : const <String, dynamic>{};
      final type = prop['type']?.toString() ?? 'string';

      if (type == 'boolean') {
        payload[key] = _boolValues[key] ?? false;
      } else if (_enumValues.containsKey(key)) {
        payload[key] = _enumValues[key];
      } else if (_isSecretProp(prop)) {
        final rawText = _controllers[key]?.text.trim() ?? '';
        if (rawText.isNotEmpty) {
          payload[key] = rawText;
        } else if (_clearedSecrets.contains(key)) {
          payload[key] = '';
        } else if (_secretFieldsSet.contains(key)) {
          payload[key] = kPluginSecretRedactedSentinel;
        }
      } else if (type == 'integer') {
        final rawText = _controllers[key]?.text.trim() ?? '';
        if (rawText.isNotEmpty) {
          final parsed = int.tryParse(rawText);
          if (parsed == null) {
            setState(() {
              _isSaving = false;
              _errorMessage = 'Field "$key" must be a whole number.';
            });
            return;
          }
          payload[key] = parsed;
        }
      } else if (type == 'number') {
        final rawText = _controllers[key]?.text.trim() ?? '';
        if (rawText.isNotEmpty) {
          final parsed = double.tryParse(rawText);
          if (parsed == null) {
            setState(() {
              _isSaving = false;
              _errorMessage = 'Field "$key" must be a valid number.';
            });
            return;
          }
          payload[key] = parsed;
        }
      } else if (_isArrayObjectCard(prop)) {
        payload[key] = _serializeArrayItems(key);
      } else {
        payload[key] = _controllers[key]?.text.trim() ?? '';
      }

      if (_requiredFields.contains(key)) {
        final v = payload[key];
        final isMissing = v == null ||
            (v is String && v.isEmpty && !_secretFieldsSet.contains(key));
        if (isMissing) {
          final label = prop['title']?.toString() ?? key;
          setState(() {
            _isSaving = false;
            _errorMessage = '$label is required.';
          });
          return;
        }
      }
    }

    try {
      await ref
          .read(pluginsProvider.notifier)
          .saveConfig(widget.plugin.name, payload);
      if (!mounted) return;
      widget.onSaved?.call();
      if (Navigator.of(context).canPop()) {
        Navigator.of(context).pop(true);
      } else {
        setState(() => _isSaving = false);
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isSaving = false;
        _errorMessage = 'Failed to save settings: $e';
      });
    }
  }

  Future<void> _invokeCommand(PluginCommandDef cmd) async {
    setState(() {
      _runningCommand = cmd.name;
      _errorMessage = null;
      _commandResultMessage = null;
    });
    try {
      final result = await ref
          .read(apiServiceProvider)
          .invokePluginCommand(widget.plugin.name, cmd.name);
      if (!mounted) return;
      final summary = result['message']?.toString() ?? jsonEncode(result);
      setState(() {
        _runningCommand = null;
        _commandResultMessage = '${cmd.displayTitle}: $summary';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _runningCommand = null;
        _errorMessage = '${cmd.displayTitle} failed: $e';
      });
    }
  }

  Widget _buildAuthSection(BuildContext context, ShelfdColors colors) {
    final authSpec = widget.plugin.auth!;
    final status = _authStatus ?? const PluginAuthStatus();
    final isLinked = status.linked;
    final nonSecretFields =
        authSpec.fields.where((f) => !f.secret).toList(growable: false);

    return Container(
      key: Key('plugin_auth_section_${widget.plugin.name}'),
      width: double.infinity,
      padding: const EdgeInsets.all(AppTokens.space12),
      margin: const EdgeInsets.only(bottom: AppTokens.space16),
      decoration: BoxDecoration(
        color: colors.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(color: colors.subtleBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                isLinked ? Icons.verified_user_outlined : Icons.link_rounded,
                size: 18,
                color: isLinked ? colors.accentSecondary : colors.accentPrimary,
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  authSpec.title ?? '${widget.plugin.displayName} Account',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              StatusBadge(
                label: isLinked ? 'Connected' : 'Not Linked',
                backgroundColor: isLinked
                    ? colors.accentSecondaryContainer
                    : colors.container,
                textColor: isLinked ? colors.accentSecondary : colors.mutedInk,
              ),
            ],
          ),
          if (authSpec.description != null &&
              authSpec.description!.trim().isNotEmpty) ...[
            const SizedBox(height: AppTokens.space4),
            Text(
              authSpec.description!,
              style: AppTypography.captionSans(color: colors.secondaryInk),
            ),
          ],
          if (status.authError != null &&
              status.authError!.trim().isNotEmpty) ...[
            const SizedBox(height: AppTokens.space12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space8),
              decoration: BoxDecoration(
                color: colors.errorBg,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: colors.errorBorder),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      status.authError!,
                      style: AppTypography.captionSans(color: colors.errorText),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  OutlinedButton.icon(
                    key: const Key('plugin_auth_reconnect_button'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.errorText,
                      side: BorderSide(color: colors.errorBorder),
                    ),
                    onPressed: _isAuthBusy
                        ? null
                        : (authSpec.supportsOAuthPkce
                            ? _handleBeginOAuth
                            : () => setState(() => _showTokenAuthFields = true)),
                    icon: const Icon(Icons.refresh_rounded, size: 15),
                    label: const Text('Reconnect'),
                  ),
                ],
              ),
            ),
          ],
          if (_authSuccessMessage != null) ...[
            const SizedBox(height: AppTokens.space8),
            Text(
              _authSuccessMessage!,
              style: AppTypography.captionSans(
                color: colors.accentSecondary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
          const SizedBox(height: AppTokens.space12),
          if (isLinked) ...[
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        status.accountLabel ??
                            'Linked to ${widget.plugin.displayName}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      if (status.clientId != null &&
                          status.clientId!.isNotEmpty)
                        Text(
                          'Client ID: ${status.clientId}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.mono(
                            fontSize: 11,
                            color: colors.mutedInk,
                          ),
                        ),
                    ],
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                OutlinedButton.icon(
                  key: const Key('plugin_auth_disconnect_button'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: colors.errorText,
                    side: BorderSide(color: colors.errorBorder),
                  ),
                  onPressed: _isAuthBusy ? null : _handleDisconnectAuth,
                  icon: const Icon(Icons.link_off_rounded, size: 16),
                  label: const Text('Disconnect'),
                ),
              ],
            ),
          ] else ...[
            ...nonSecretFields.map((field) {
              return Padding(
                padding: const EdgeInsets.only(bottom: AppTokens.space8),
                child: TextFormField(
                  key: Key('plugin_auth_field_${field.key}'),
                  controller: _authControllers[field.key],
                  style: AppTypography.bodySans(fontSize: 13, color: colors.ink),
                  decoration: InputDecoration(
                    labelText: field.title,
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
              );
            }),
            if (authSpec.supportsOAuthPkce) ...[
              Wrap(
                spacing: AppTokens.space8,
                runSpacing: AppTokens.space8,
                children: [
                  if (!kIsWeb)
                    FilledButton.icon(
                      key: const Key('plugin_auth_open_web_dashboard_button'),
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.accentPrimary,
                        foregroundColor: colors.onAccent,
                      ),
                      onPressed: _isAuthBusy
                          ? null
                          : _handleOpenWebDashboardForAuth,
                      icon: const Icon(Icons.open_in_browser_rounded, size: 16),
                      label: const Text('Open Web Dashboard to Link Account'),
                    ),
                  if (kIsWeb)
                    FilledButton.icon(
                      key: const Key('plugin_auth_oauth_begin_button'),
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.accentPrimary,
                        foregroundColor: colors.onAccent,
                      ),
                      onPressed: _isAuthBusy ? null : _handleBeginOAuth,
                      icon: const Icon(Icons.open_in_new_rounded, size: 16),
                      label: const Text('Connect with OAuth (PKCE)'),
                    )
                  else
                    OutlinedButton.icon(
                      key: const Key('plugin_auth_oauth_begin_button'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: colors.ink,
                        side: BorderSide(color: colors.border),
                      ),
                      onPressed: _isAuthBusy ? null : _handleBeginOAuth,
                      icon: const Icon(Icons.open_in_new_rounded, size: 16),
                      label: const Text('Connect with OAuth (PKCE)'),
                    ),
                  if (authSpec.supportsToken)
                    TextButton.icon(
                      key: const Key('plugin_auth_toggle_token_button'),
                      onPressed: () {
                        setState(() {
                          _showTokenAuthFields = !_showTokenAuthFields;
                        });
                      },
                      icon: Icon(
                        _showTokenAuthFields
                            ? Icons.expand_less_rounded
                            : Icons.expand_more_rounded,
                        size: 18,
                      ),
                      label: Text(
                        _showTokenAuthFields
                            ? 'Hide Token Entry'
                            : 'Use Access Token Instead',
                      ),
                    ),
                ],
              ),
              const SizedBox(height: AppTokens.space8),
              Row(
                children: [
                  Expanded(
                    child: TextFormField(
                      key: const Key('plugin_auth_code_field'),
                      controller: _authCodeController,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        color: colors.ink,
                      ),
                      decoration: InputDecoration(
                        labelText: 'Authorization Code',
                        hintText: 'Paste ?code=... from callback redirect',
                        isDense: true,
                        border: OutlineInputBorder(
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                          borderSide: BorderSide(color: colors.border),
                        ),
                        enabledBorder: OutlineInputBorder(
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                          borderSide: BorderSide(color: colors.border),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  OutlinedButton(
                    key: const Key('plugin_auth_complete_code_button'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.ink,
                      side: BorderSide(color: colors.border),
                    ),
                    onPressed: _isAuthBusy ? null : _handleCompleteCodeAuth,
                    child: const Text('Link Code'),
                  ),
                ],
              ),
            ],
            if (authSpec.supportsToken &&
                (_showTokenAuthFields || !authSpec.supportsOAuthPkce)) ...[
              const SizedBox(height: AppTokens.space12),
              TextFormField(
                key: const Key('plugin_auth_field_access_token'),
                controller: _authControllers['access_token'],
                style: AppTypography.mono(fontSize: 12, color: colors.ink),
                decoration: InputDecoration(
                  labelText: 'Access Token',
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
              const SizedBox(height: AppTokens.space8),
              TextFormField(
                key: const Key('plugin_auth_field_refresh_token'),
                controller: _authControllers['refresh_token'],
                style: AppTypography.mono(fontSize: 12, color: colors.ink),
                decoration: InputDecoration(
                  labelText: 'Refresh Token (Optional)',
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
              const SizedBox(height: AppTokens.space8),
              Align(
                alignment: Alignment.centerRight,
                child: OutlinedButton.icon(
                  key: const Key('plugin_auth_complete_token_button'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: colors.ink,
                    side: BorderSide(color: colors.border),
                  ),
                  onPressed: _isAuthBusy ? null : _handleCompleteTokenAuth,
                  icon: const Icon(Icons.check_rounded, size: 16),
                  label: const Text('Save Token'),
                ),
              ),
            ],
          ],
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final props = _visibleProperties;

    return AlertDialog(
      backgroundColor: colors.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: BorderSide(color: colors.border),
      ),
      title: Row(
        children: [
          Icon(Icons.tune_rounded, size: 20, color: colors.accentPrimary),
          const SizedBox(width: AppTokens.space8),
          Expanded(
            child: Text(
              'Configure ${widget.plugin.displayName}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
            ),
          ),
        ],
      ),
      content: SizedBox(
        width: 520,
        child: _isLoading
            ? const Padding(
                padding: EdgeInsets.symmetric(vertical: AppTokens.space24),
                child: Center(child: CircularProgressIndicator()),
              )
            : SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (widget.plugin.permissions.networkDomains.isNotEmpty) ...[
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(AppTokens.space12),
                        decoration: BoxDecoration(
                          color: colors.containerLow,
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                          border: Border.all(color: colors.subtleBorder),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Network domains used by this plugin',
                              style: AppTypography.bodySans(
                                fontSize: 12,
                                fontWeight: FontWeight.w600,
                                color: colors.secondaryInk,
                              ),
                            ),
                            const SizedBox(height: AppTokens.space4),
                            Text(
                              widget.plugin.permissions.networkDomains
                                  .join(', '),
                              style: AppTypography.mono(
                                fontSize: 11,
                                color: colors.mutedInk,
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: AppTokens.space16),
                    ],
                    if (_errorMessage != null) ...[
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(AppTokens.space12),
                        decoration: BoxDecoration(
                          color: colors.errorBg,
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
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
                      const SizedBox(height: AppTokens.space12),
                    ],
                    if (_commandResultMessage != null) ...[
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(AppTokens.space12),
                        decoration: BoxDecoration(
                          color: colors.accentSecondaryContainer,
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                        ),
                        child: Text(
                          _commandResultMessage!,
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            color: colors.accentSecondary,
                          ),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space12),
                    ],
                    if (widget.plugin.hasAuth)
                      _buildAuthSection(context, colors),
                    if (props.isEmpty && !widget.plugin.hasAuth)
                      Text(
                        'This plugin has no user-configurable settings.',
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          color: colors.mutedInk,
                        ),
                      )
                    else
                      ...props.entries.map((entry) {
                        final key = entry.key;
                        final prop = entry.value is Map
                            ? Map<String, dynamic>.from(entry.value as Map)
                            : const <String, dynamic>{};
                        if (_isArrayObjectCard(prop)) {
                          return _buildArraySection(context, colors, key, prop);
                        }
                        return Padding(
                          padding:
                              const EdgeInsets.only(bottom: AppTokens.space16),
                          child: _buildField(context, colors, key, prop),
                        );
                      }),
                    if (widget.plugin.commands.isNotEmpty) ...[
                      Divider(color: colors.subtleBorder, height: 1),
                      const SizedBox(height: AppTokens.space12),
                      Text(
                        'Plugin Actions',
                        style: AppTypography.bodySans(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: colors.secondaryInk,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Wrap(
                        spacing: AppTokens.space8,
                        runSpacing: AppTokens.space8,
                        children: widget.plugin.commands.map((cmd) {
                          final isBusy = _runningCommand == cmd.name;
                          return OutlinedButton.icon(
                            key: Key('plugin_cmd_${cmd.name}'),
                            style: OutlinedButton.styleFrom(
                              foregroundColor: colors.ink,
                              side: BorderSide(color: colors.border),
                            ),
                            onPressed:
                                _runningCommand != null ? null : () => _invokeCommand(cmd),
                            icon: isBusy
                                ? SizedBox(
                                    width: 14,
                                    height: 14,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      color: colors.accentPrimary,
                                    ),
                                  )
                                : const Icon(Icons.play_arrow_rounded, size: 16),
                            label: Text(cmd.displayTitle),
                          );
                        }).toList(),
                      ),
                    ],
                  ],
                ),
              ),
      ),
      actions: [
        TextButton(
          onPressed: _isSaving ? null : () => Navigator.of(context).pop(false),
          child: Text(
            'Cancel',
            style: AppTypography.bodySans(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: colors.secondaryInk,
            ),
          ),
        ),
        if (props.isNotEmpty)
          FilledButton(
            key: const Key('plugin_config_save_button'),
            style: FilledButton.styleFrom(
              backgroundColor: colors.accentPrimary,
              foregroundColor: colors.onAccent,
            ),
            onPressed: (_isLoading || _isSaving) ? null : _handleSave,
            child: Text(_isSaving ? 'Saving...' : 'Save Settings'),
          ),
      ],
    );
  }

  /// Builds a repeatable object-array section with Add/Remove controls.
  Widget _buildArraySection(
    BuildContext context,
    ShelfdColors colors,
    String key,
    Map<String, dynamic> prop,
  ) {
    final title = prop['title']?.toString() ?? key;
    final description = prop['description']?.toString();
    final itemProps = _itemProperties(prop);
    final items = _arrayItems[key] ?? [];
    final isRequired = _requiredFields.contains(key);

    return Padding(
      padding: const EdgeInsets.only(bottom: AppTokens.space16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppTokens.space8),
            decoration: BoxDecoration(
              color: colors.containerLow,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: colors.subtleBorder),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    if (isRequired)
                      Text(
                        '*',
                        style: AppTypography.bodySans(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: colors.errorText,
                        ),
                      ),
                  ],
                ),
                if (description != null && description.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4.0),
                    child: Text(
                      description,
                      style:
                          AppTypography.captionSans(color: colors.mutedInk),
                    ),
                  ),
                const SizedBox(height: AppTokens.space4),
                // Render each array item card.
                ...items.map((item) {
                  final idx = items.indexOf(item);
                  return _buildArrayItemCard(
                    context,
                    colors,
                    key,
                    itemProps,
                    idx,
                  );
                }),
              ],
            ),
          ),
          const SizedBox(height: AppTokens.space4),
          // Add button.
          OutlinedButton.icon(
            key: Key('array_add_$key'),
            onPressed: () => _addArrayItem(key),
            icon: const Icon(Icons.add_rounded, size: 18),
            label: Text('Add $title'),
          ),
        ],
      ),
    );
  }

  /// Renders a single array-item card with nested field widgets and a remove button.
  Widget _buildArrayItemCard(
    BuildContext context,
    ShelfdColors colors,
    String key,
    Map<String, dynamic> itemProps,
    int idx,
  ) {
    return Padding(
      padding: const EdgeInsets.only(top: AppTokens.space8),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(AppTokens.space8),
        decoration: BoxDecoration(
          color: colors.surface,
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          border: Border.all(color: colors.border),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  'Item ${idx + 1}',
                  style: AppTypography.titleSerif(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
                const Spacer(),
                IconButton(
                  key: Key('array_remove_$key.$idx'),
                  icon: Icon(Icons.delete_outline, size: 20, color: colors.errorText),
                  onPressed: () => _removeArrayItem(key, idx),
                  tooltip: 'Remove item',
                ),
              ],
            ),
            // Render each field within the array item.
            ...itemProps.entries.map((propEntry) {
              final fieldKey = propEntry.key;
              final fieldProp = propEntry.value is Map
                  ? Map<String, dynamic>.from(propEntry.value as Map)
                  : <String, dynamic>{};
              return _buildArrayItemField(
                context,
                colors,
                key,
                idx,
                fieldKey,
                fieldProp,
              );
            }),
          ],
        ),
      ),
    );
  }

  /// Renders a single nested field inside an array item card.
  Widget _buildArrayItemField(
    BuildContext context,
    ShelfdColors colors,
    String key,
    int idx,
    String fieldKey,
    Map<String, dynamic> fieldProp,
  ) {
    final title = fieldProp['title']?.toString() ?? fieldKey;
    final description = fieldProp['description']?.toString();
    final type = fieldProp['type']?.toString() ?? 'string';
    final ctrlKey = '$key.$idx.$fieldKey';
    final controller = _itemControllers[ctrlKey];
    final isSecret = _isSecretProp(fieldProp);
    final isObscured = controller != null &&
        (controller.text.isEmpty || !_setItemSecretsLoaded.contains('$key.items[$idx].$fieldKey'));

    return Padding(
      padding: const EdgeInsets.only(top: 6.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: AppTypography.bodySans(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          if (description != null && description.isNotEmpty)
            Text(
              description,
              style: AppTypography.captionSans(color: colors.secondaryInk),
            ),
          const SizedBox(height: 4.0),
          if (type == 'boolean') ...[
            Switch(
              value: _itemControllers['$key.$idx.$fieldKey']?.text == 'true',
              onChanged: (val) => setState(() {
                _itemControllers['$key.$idx.$fieldKey']?.text = val.toString();
              }),
            ),
          ] else if (fieldProp['enum'] is List &&
              (fieldProp['enum'] as List).isNotEmpty) ...[
            DropdownButtonHideUnderline(
              child: ButtonTheme(
                alignedDropdown: true,
                child: DropdownButton<String>(
                  value: _itemControllers['$key.$idx.$fieldKey']?.text,
                  isExpanded: true,
                  items: ((fieldProp['enum'] as List)
                              .map((e) => e.toString())
                              .toList())
                          .map((v) => DropdownMenuItem(
                                value: v,
                                child: Text(v),
                              ))
                          .toList(),
                  onChanged: (val) {
                    if (val != null) setState(() => controller!.text = val);
                  },
                ),
              ),
            ),
          ] else ...[
            TextField(
              controller: controller ?? TextEditingController(),
              obscureText: isObscured && isSecret,
              decoration: InputDecoration(
                hintText: 'Enter $title',
                filled: true,
                fillColor: colors.surface,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  borderSide: BorderSide(color: colors.border),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  borderSide: BorderSide(color: colors.border),
                ),
                contentPadding: EdgeInsets.symmetric(
                  horizontal: AppTokens.space8,
                  vertical: 6.0,
                ),
              ),
              style: AppTypography.bodySans(fontSize: 13, color: colors.ink),
              onChanged: (val) => setState(() => controller!.text = val),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildField(
    BuildContext context,
    ShelfdColors colors,
    String key,
    Map<String, dynamic> prop,
  ) {
    final title = prop['title']?.toString() ?? key;
    final description = prop['description']?.toString();
    final type = prop['type']?.toString() ?? 'string';
    final isRequired = _requiredFields.contains(key);
    final isSecret = _isSecretProp(prop);
    final supportsCurl = _supportsCurlPaste(prop);
    final isConfigured =
        _secretFieldsSet.contains(key) && !_clearedSecrets.contains(key);

    if (type == 'boolean') {
      return Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppTokens.space12,
          vertical: AppTokens.space8,
        ),
        decoration: BoxDecoration(
          color: colors.containerLow,
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          border: Border.all(color: colors.subtleBorder),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: colors.ink,
                    ),
                  ),
                  if (description != null && description.isNotEmpty)
                    Text(
                      description,
                      style: AppTypography.captionSans(color: colors.mutedInk),
                    ),
                ],
              ),
            ),
            const SizedBox(width: AppTokens.space8),
            Switch(
              key: Key('plugin_field_$key'),
              value: _boolValues[key] ?? false,
              activeTrackColor: colors.accentPrimary,
              onChanged: (val) => setState(() => _boolValues[key] = val),
            ),
          ],
        ),
      );
    }

    if (_enumValues.containsKey(key)) {
      final options = (prop['enum'] as List).map((e) => e.toString()).toList();
      final selected = options.contains(_enumValues[key])
          ? _enumValues[key]!
          : options.first;
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            isRequired ? '$title *' : title,
            style: AppTypography.bodySans(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          if (description != null && description.isNotEmpty) ...[
            const SizedBox(height: AppTokens.space4),
            Text(
              description,
              style: AppTypography.captionSans(color: colors.mutedInk),
            ),
          ],
          const SizedBox(height: AppTokens.space8),
          DropdownButtonHideUnderline(
            child: Container(
              width: double.infinity,
              padding:
                  const EdgeInsets.symmetric(horizontal: AppTokens.space12),
              decoration: BoxDecoration(
                color: colors.containerLow,
                border: Border.all(color: colors.border),
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              ),
              child: DropdownButton<String>(
                key: Key('plugin_field_$key'),
                value: selected,
                isExpanded: true,
                dropdownColor: colors.surface,
                style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                items: options
                    .map((opt) => DropdownMenuItem<String>(
                          value: opt,
                          child: Text(opt),
                        ))
                    .toList(),
                onChanged: (val) {
                  if (val != null) {
                    setState(() => _enumValues[key] = val);
                  }
                },
              ),
            ),
          ),
        ],
      );
    }

    final isObscured = isSecret && _obscuredKeys.contains(key);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                isRequired ? '$title *' : title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
            ),
            if (isSecret && isConfigured) ...[
              const SizedBox(width: AppTokens.space8),
              StatusBadge(
                label: 'Configured',
                backgroundColor: colors.accentSecondaryContainer,
                textColor: colors.accentSecondary,
              ),
              const SizedBox(width: AppTokens.space4),
              TextButton(
                key: Key('plugin_clear_secret_$key'),
                onPressed: () {
                  setState(() {
                    _clearedSecrets.add(key);
                    _controllers[key]?.clear();
                  });
                },
                child: Text(
                  'Clear',
                  style: AppTypography.captionSans(color: colors.errorText),
                ),
              ),
            ],
            if (supportsCurl) ...[
              const SizedBox(width: AppTokens.space8),
              OutlinedButton.icon(
                key: Key('plugin_paste_curl_$key'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space8,
                    vertical: AppTokens.space4,
                  ),
                ),
                onPressed: () => _pasteFromClipboard(key),
                icon: const Icon(Icons.content_paste_rounded, size: 14),
                label: const Text('Paste cURL'),
              ),
            ],
          ],
        ),
        if (description != null && description.isNotEmpty) ...[
          const SizedBox(height: AppTokens.space4),
          Text(
            description,
            style: AppTypography.captionSans(color: colors.mutedInk),
          ),
        ],
        if (supportsCurl) ...[
          const SizedBox(height: AppTokens.space4),
          Text(
            'Tip: Paste a raw cookie/token or a full browser DevTools "Copy as cURL" command. Shelfd extracts and encrypts the credential on the server.',
            style: AppTypography.captionSans(color: colors.secondaryInk),
          ),
        ],
        const SizedBox(height: AppTokens.space8),
        TextFormField(
          key: Key('plugin_field_$key'),
          controller: _controllers[key],
          obscureText: isObscured,
          maxLines: isObscured ? 1 : (supportsCurl ? 3 : 1),
          minLines: 1,
          keyboardType: (type == 'integer' || type == 'number')
              ? TextInputType.number
              : TextInputType.text,
          style: isSecret
              ? AppTypography.mono(fontSize: 12, color: colors.ink)
              : AppTypography.bodySans(fontSize: 14, color: colors.ink),
          onChanged: (_) {
            if (_clearedSecrets.contains(key)) {
              setState(() => _clearedSecrets.remove(key));
            }
          },
          decoration: InputDecoration(
            hintText: (isSecret && isConfigured)
                ? 'Encrypted credential stored. Enter a new value to replace.'
                : (prop['x-shelfd-cookie-name'] != null
                    ? 'Paste ${prop['x-shelfd-cookie-name']} cookie or cURL command'
                    : null),
            isDense: true,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              borderSide: BorderSide(color: colors.border),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              borderSide: BorderSide(color: colors.border),
            ),
            suffixIcon: isSecret
                ? IconButton(
                    tooltip: isObscured ? 'Show value' : 'Hide value',
                    icon: Icon(
                      isObscured
                          ? Icons.visibility_outlined
                          : Icons.visibility_off_outlined,
                      size: 18,
                      color: colors.mutedInk,
                    ),
                    onPressed: () {
                      setState(() {
                        if (isObscured) {
                          _obscuredKeys.remove(key);
                        } else {
                          _obscuredKeys.add(key);
                        }
                      });
                    },
                  )
                : null,
          ),
        ),
      ],
    );
  }
}

class PluginLogsDialog extends ConsumerStatefulWidget {
  final PluginSummary plugin;

  const PluginLogsDialog({
    super.key,
    required this.plugin,
  });

  @override
  ConsumerState<PluginLogsDialog> createState() => _PluginLogsDialogState();
}

class _PluginLogsDialogState extends ConsumerState<PluginLogsDialog> {
  PluginDetail? _detail;
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadDetail();
  }

  Future<void> _loadDetail() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final detail = await ref
          .read(pluginsProvider.notifier)
          .fetchDetail(widget.plugin.name);
      if (!mounted) return;
      setState(() {
        _detail = detail;
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _error = 'Failed to fetch plugin diagnostics: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final detail = _detail;

    return AlertDialog(
      backgroundColor: colors.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: BorderSide(color: colors.border),
      ),
      title: Row(
        children: [
          Icon(Icons.terminal_rounded, size: 20, color: colors.accentPrimary),
          const SizedBox(width: AppTokens.space8),
          Expanded(
            child: Text(
              '${widget.plugin.displayName} Diagnostics',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
            ),
          ),
          IconButton(
            tooltip: 'Refresh logs',
            onPressed: _isLoading ? null : _loadDetail,
            icon: const Icon(Icons.refresh_rounded, size: 18),
          ),
        ],
      ),
      content: SizedBox(
        width: 580,
        height: 380,
        child: _isLoading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(
                    child: Text(
                      _error!,
                      style: AppTypography.bodySans(
                        fontSize: 13,
                        color: colors.errorText,
                      ),
                    ),
                  )
                : Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (detail?.summary.lastError != null &&
                          detail!.summary.lastError!.isNotEmpty) ...[
                        Container(
                          width: double.infinity,
                          padding: const EdgeInsets.all(AppTokens.space8),
                          decoration: BoxDecoration(
                            color: colors.errorBg,
                            borderRadius:
                                BorderRadius.circular(AppTokens.radiusSm),
                            border: Border.all(color: colors.errorBorder),
                          ),
                          child: Text(
                            'Last error: ${detail.summary.lastError}',
                            style: AppTypography.mono(
                              fontSize: 11,
                              color: colors.errorText,
                            ),
                          ),
                        ),
                        const SizedBox(height: AppTokens.space8),
                      ],
                      Expanded(
                        child: Container(
                          width: double.infinity,
                          padding: const EdgeInsets.all(AppTokens.space12),
                          decoration: BoxDecoration(
                            color: colors.containerLow,
                            borderRadius:
                                BorderRadius.circular(AppTokens.radiusSm),
                            border: Border.all(color: colors.border),
                          ),
                          child: (detail == null || detail.logs.isEmpty)
                              ? Center(
                                  child: Text(
                                    'No stderr log output recorded yet.',
                                    style: AppTypography.mono(
                                      fontSize: 12,
                                      color: colors.mutedInk,
                                    ),
                                  ),
                                )
                              : ListView.builder(
                                  itemCount: detail.logs.length,
                                  itemBuilder: (context, index) {
                                    return Padding(
                                      padding: const EdgeInsets.only(bottom: 4),
                                      child: SelectableText(
                                        detail.logs[index],
                                        style: AppTypography.mono(
                                          fontSize: 11,
                                          color: colors.ink,
                                        ),
                                      ),
                                    );
                                  },
                                ),
                        ),
                      ),
                    ],
                  ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(
            'Close',
            style: AppTypography.bodySans(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: colors.secondaryInk,
            ),
          ),
        ),
      ],
    );
  }
}
