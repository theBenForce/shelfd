import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/plugin.dart';
import '../../core/app_shell.dart';
import '../../core/primitives.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'plugin_config_dialog.dart';

class SettingsView extends ConsumerStatefulWidget {
  const SettingsView({super.key});

  @override
  ConsumerState<SettingsView> createState() => _SettingsViewState();
}

class _SettingsViewState extends ConsumerState<SettingsView> {
  final _passwordFormKey = GlobalKey<FormState>();
  final _currentPasswordController = TextEditingController();
  final _newPasswordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();
  final _defaultUsernameController = TextEditingController();

  bool _showPasswordForm = false;
  bool _obscureCurrent = true;
  bool _obscureNew = true;
  bool _obscureConfirm = true;

  bool _isChangingPassword = false;
  String? _passwordError;
  String? _passwordSuccess;
  String? _usernameSavedMessage;

  @override
  void initState() {
    super.initState();
    final storage = ref.read(storageServiceProvider);
    final savedUsername = storage.getSavedUsername();
    _defaultUsernameController.text = savedUsername ?? '';

    final callbackCode = Uri.base.queryParameters['code'];
    if (callbackCode != null && callbackCode.trim().isNotEmpty) {
      final callbackPlugin =
          Uri.base.queryParameters['plugin']?.trim() ??
          Uri.base.queryParameters['state']?.trim() ??
          storage.getActivePkcePlugin() ??
          'yoto';
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _handlePluginOAuthCallback(callbackPlugin, callbackCode.trim());
      });
    }
  }

  @override
  void dispose() {
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    _defaultUsernameController.dispose();
    super.dispose();
  }

  String _resolvePluginRedirectUri(String pluginName) {
    final base = Uri.base;
    final encoded = Uri.encodeComponent(pluginName);
    if (base.scheme == 'http' || base.scheme == 'https') {
      return '${base.origin}/settings?plugin=$encoded';
    }
    return 'http://localhost:8080/settings?plugin=$encoded';
  }

  Future<void> _handlePluginOAuthCallback(
    String pluginName,
    String code,
  ) async {
    if (pluginName.isEmpty || code.isEmpty) return;
    final storage = ref.read(storageServiceProvider);
    final verifier = storage.getPluginPkceVerifier(pluginName) ?? '';
    try {
      final api = ref.read(apiServiceProvider);
      await api.completePluginAuth(pluginName, {
        'method': 'oauth_pkce',
        'code': code,
        if (verifier.isNotEmpty) 'code_verifier': verifier,
        'redirect_uri': _resolvePluginRedirectUri(pluginName),
      });
      await storage.clearPluginPkceVerifier(pluginName);
      await ref.read(pluginsProvider.notifier).loadPlugins(silent: true);
      if (mounted) {
        final summary = ref.read(pluginsProvider).findByName(pluginName);
        final display = summary?.displayName ?? pluginName;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$display account linked!')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to link $pluginName account: $e')),
        );
      }
    }
  }

  void _onNavTapped(int index) {
    switch (index) {
      case 0:
        context.go('/books');
      case 1:
        context.go('/search');
      case 2:
        context.go('/uploads');
      case 3:
        context.go('/utilities');
      case 4:
        context.go('/settings');
    }
  }

  Future<void> _handleSaveDefaultUsername() async {
    final newDefault = _defaultUsernameController.text.trim();
    if (newDefault.isEmpty) return;

    final storage = ref.read(storageServiceProvider);
    await storage.saveSavedUsername(newDefault);
    setState(() {
      _usernameSavedMessage = 'Default username updated to "$newDefault"';
    });
  }

  Future<void> _handleChangePassword() async {
    setState(() {
      _passwordError = null;
      _passwordSuccess = null;
    });

    if (!_passwordFormKey.currentState!.validate()) return;

    final currentPass = _currentPasswordController.text;
    final newPass = _newPasswordController.text;

    setState(() => _isChangingPassword = true);

    try {
      await ref
          .read(authProvider.notifier)
          .changePassword(currentPass, newPass);
      if (mounted) {
        setState(() {
          _isChangingPassword = false;
          _passwordSuccess = 'Password updated successfully!';
          _currentPasswordController.clear();
          _newPasswordController.clear();
          _confirmPasswordController.clear();
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Password updated successfully!'),
            backgroundColor: AppTokens.libraryGreen,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isChangingPassword = false;
          final msg = e.toString();
          if (msg.contains('ApiException')) {
            final colon = msg.indexOf(':');
            _passwordError =
                colon != -1 ? msg.substring(colon + 1).trim() : msg;
          } else {
            _passwordError = msg;
          }
        });
      }
    }
  }

  Future<void> _handleLogout() async {
    await ref.read(authProvider.notifier).logout();
    if (mounted) {
      context.go('/connect');
    }
  }

  Future<void> _handleInstallPluginZip() async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['zip'],
        allowMultiple: false,
      );
      if (result.isEmpty) return;
      final file = result.first;
      final bytes = await file.readAsBytes();
      if (bytes.isEmpty || !mounted) return;
      await ref.read(pluginsProvider.notifier).installZip(
            filename: file.name,
            bytes: bytes,
          );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to read plugin archive: $e')),
        );
      }
    }
  }

  Future<void> _handleUpdatePluginZip(PluginSummary plugin) async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['zip'],
        allowMultiple: false,
      );
      if (result.isEmpty) return;
      final file = result.first;
      final bytes = await file.readAsBytes();
      if (bytes.isEmpty || !mounted) return;
      await ref.read(pluginsProvider.notifier).updateZip(
            name: plugin.name,
            filename: file.name,
            bytes: bytes,
          );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to read plugin archive: $e')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);
    final currentUsername = authState.user?.username ?? 'Connected User';
    final colors = context.shelfdColors;

    final hasAppShell =
        context.findAncestorWidgetOfExactType<AppShell>() != null;

    final topBar = isDesktop
        ? null
        : const ShelfdTopBar(
            title: 'Settings',
            subtitle: 'Appearance, plugins, server & account',
          );

    final bodyContent = Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: AppTokens.maxReadingWidth),
        child: ListView(
          cacheExtent: 1200,
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPad,
            vertical: AppTokens.space24,
          ),
          children: [
            if (isDesktop) ...[
              const ShelfdPageHeader(
                title: 'Settings',
                subtitle: 'Appearance, plugins, server & account',
              ),
              const SizedBox(height: AppTokens.space24),
            ],

            // 1. Appearance & Theme
            _buildAppearanceCard(),
            const SizedBox(height: AppTokens.space20),

            // 2. Plugins & Storefronts
            Consumer(
              builder: (context, ref, _) {
                final pluginsState = ref.watch(pluginsProvider);
                return _buildPluginsCard(context, context.shelfdColors, pluginsState);
              },
            ),
            const SizedBox(height: AppTokens.space20),

            // 3. Server Connection & AI Indexing
            _buildServerCard(authState.serverUrl),
            const SizedBox(height: AppTokens.space20),

            // 4. Account & Security (with progressive disclosure for password)
            _buildAccountCard(colors, currentUsername),
            const SizedBox(height: AppTokens.space32),
          ],
        ),
      ),
    );

    if (hasAppShell) {
      return Scaffold(
        backgroundColor: Colors.transparent,
        appBar: topBar,
        body: bodyContent,
      );
    }

    return ShelfdAdaptiveScaffold(
      currentIndex: 4,
      onNavTap: _onNavTapped,
      currentPath: '/settings',
      onNavigate: (path) => context.go(path),
      appBar: topBar,
      body: bodyContent,
    );
  }

  Widget _buildAppearanceCard() {
    return Consumer(
      builder: (context, ref, _) {
        final readerSettings = ref.watch(readerSettingsProvider);
        final c = context.shelfdColors;
        return ShelfdSectionCard(
          icon: Icons.palette_outlined,
          title: 'Appearance & Theme',
          subtitle: 'Choose the interface and reading palette across Shelfd.',
          child: SizedBox(
            width: double.infinity,
            child: SegmentedButton<ReadingThemeMode>(
              style: ButtonStyle(
                backgroundColor: WidgetStateProperty.resolveWith(
                  (states) => states.contains(WidgetState.selected)
                      ? c.accentContainer
                      : c.containerLow,
                ),
                foregroundColor: WidgetStateProperty.resolveWith(
                  (states) => states.contains(WidgetState.selected)
                      ? c.accentPrimary
                      : c.ink,
                ),
                side: WidgetStatePropertyAll(BorderSide(color: c.border)),
              ),
              segments: const [
                ButtonSegment(
                  value: ReadingThemeMode.bone,
                  label: Text('Bone'),
                  icon: Icon(Icons.light_mode_outlined),
                ),
                ButtonSegment(
                  value: ReadingThemeMode.sepia,
                  label: Text('Sepia'),
                  icon: Icon(Icons.menu_book_outlined),
                ),
                ButtonSegment(
                  value: ReadingThemeMode.dark,
                  label: Text('Dark'),
                  icon: Icon(Icons.dark_mode_outlined),
                ),
              ],
              selected: {readerSettings.themeMode},
              onSelectionChanged: (set) {
                if (set.isNotEmpty) {
                  ref
                      .read(readerSettingsProvider.notifier)
                      .setThemeMode(set.first);
                }
              },
            ),
          ),
        );
      },
    );
  }

  Widget _buildPluginsCard(
    BuildContext context,
    ShelfdColors c,
    PluginsState pluginsState,
  ) {
    return ShelfdSectionCard(
      key: const Key('plugins_settings_card'),
      icon: Icons.extension_outlined,
      title: 'Plugins & Storefronts',
      subtitle:
          'Connect external book storefronts, outbound actions, or MCP tools.',
      trailing: Wrap(
        spacing: AppTokens.space8,
        runSpacing: AppTokens.space8,
        children: [
          ShelfdSecondaryButton(
            key: const Key('plugins_rescan_button'),
            label: 'Rescan',
            icon: Icons.refresh_rounded,
            compact: true,
            onPressed: pluginsState.isLoading
                ? null
                : () => ref.read(pluginsProvider.notifier).discover(),
          ),
          FilledButton.icon(
            key: const Key('plugins_install_zip_button'),
            style: FilledButton.styleFrom(
              backgroundColor: c.accentPrimary,
              foregroundColor: c.onAccent,
              padding: const EdgeInsets.symmetric(
                horizontal: AppTokens.space12,
                vertical: AppTokens.space8,
              ),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              ),
            ),
            onPressed: pluginsState.isBusyInstallingOrUpdating
                ? null
                : _handleInstallPluginZip,
            icon: const Icon(Icons.upload_file_rounded, size: 15),
            label: Text(
              pluginsState.isInstalling ? 'Installing...' : 'Install .zip',
              style: AppTypography.bodySans(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: c.onAccent,
              ),
            ),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (pluginsState.errorMessage != null) ...[
            ShelfdInlineBanner(
              message: pluginsState.errorMessage!,
              tone: ShelfdBannerTone.error,
            ),
            const SizedBox(height: AppTokens.space12),
          ],
          if (pluginsState.successMessage != null) ...[
            ShelfdInlineBanner(
              message: pluginsState.successMessage!,
              tone: ShelfdBannerTone.success,
            ),
            const SizedBox(height: AppTokens.space12),
          ],
          if (pluginsState.plugins.isEmpty)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space16),
              decoration: BoxDecoration(
                color: c.containerLow,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: c.subtleBorder),
              ),
              child: Text(
                pluginsState.isLoading
                    ? 'Loading installed plugins...'
                    : 'No plugins installed yet. Upload a .zip archive or add plugins to your server plugins directory.',
                style: AppTypography.bodySans(fontSize: 13, color: c.mutedInk),
              ),
            )
          else
            ...pluginsState.plugins.asMap().entries.map((entry) {
              final isLast = entry.key == pluginsState.plugins.length - 1;
              return Padding(
                padding: EdgeInsets.only(bottom: isLast ? 0 : AppTokens.space12),
                child: _buildPluginRowCard(
                  context,
                  c,
                  entry.value,
                  pluginsState,
                ),
              );
            }),
        ],
      ),
    );
  }

  Widget _buildPluginRowCard(
    BuildContext context,
    ShelfdColors c,
    PluginSummary plugin,
    PluginsState pluginsState,
  ) {
    final isHealthy = plugin.status == 'running' || plugin.status == 'idle';
    final isError = plugin.status == 'unhealthy';
    final badgeBg = isError
        ? c.errorBg
        : (isHealthy ? c.accentSecondaryContainer : c.container);
    final badgeFg = isError
        ? c.errorText
        : (isHealthy ? c.accentSecondary : c.mutedInk);

    return Container(
      key: Key('plugin_card_${plugin.name}'),
      padding: const EdgeInsets.all(AppTokens.space12),
      decoration: BoxDecoration(
        color: c.containerLow,
        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
        border: Border.all(color: c.subtleBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '${plugin.displayName} v${plugin.version}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: c.ink,
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              StatusBadge(
                label: plugin.status,
                backgroundColor: badgeBg,
                textColor: badgeFg,
              ),
              const SizedBox(width: AppTokens.space8),
              Switch(
                key: Key('plugin_toggle_${plugin.name}'),
                value: plugin.enabled,
                onChanged: (val) => ref
                    .read(pluginsProvider.notifier)
                    .setEnabled(plugin.name, val),
              ),
            ],
          ),
          if (plugin.description != null &&
              plugin.description!.trim().isNotEmpty) ...[
            const SizedBox(height: AppTokens.space4),
            Text(
              plugin.description!,
              style: AppTypography.captionSans(color: c.secondaryInk),
            ),
          ],
          if (plugin.permissions.networkDomains.isNotEmpty) ...[
            const SizedBox(height: AppTokens.space4),
            Text(
              'Domains: ${plugin.permissions.networkDomains.join(', ')}',
              style: AppTypography.mono(fontSize: 11, color: c.mutedInk),
            ),
          ],
          if (plugin.lastError != null && plugin.lastError!.isNotEmpty) ...[
            const SizedBox(height: AppTokens.space4),
            Text(
              plugin.lastError!,
              style: AppTypography.mono(fontSize: 11, color: c.errorText),
            ),
          ],
          const SizedBox(height: AppTokens.space12),
          Wrap(
            spacing: AppTokens.space8,
            runSpacing: AppTokens.space8,
            children: [
              if (plugin.hasConfigSchema ||
                  plugin.commands.isNotEmpty ||
                  plugin.hasAuth)
                ShelfdSecondaryButton(
                  key: Key('plugin_configure_${plugin.name}'),
                  label:
                      !plugin.configured ? 'Configure (Required)' : 'Configure',
                  icon: Icons.tune_rounded,
                  compact: true,
                  onPressed: () {
                    showDialog<bool>(
                      context: context,
                      builder: (_) => PluginConfigDialog(plugin: plugin),
                    );
                  },
                ),
              ShelfdSecondaryButton(
                key: Key('plugin_update_${plugin.name}'),
                label: pluginsState.updatingPluginName == plugin.name
                    ? 'Updating...'
                    : 'Update .zip',
                icon: Icons.system_update_alt_rounded,
                compact: true,
                onPressed: pluginsState.isBusyInstallingOrUpdating
                    ? null
                    : () => _handleUpdatePluginZip(plugin),
              ),
              ShelfdSecondaryButton(
                key: Key('plugin_logs_${plugin.name}'),
                label: 'Logs',
                icon: Icons.terminal_rounded,
                compact: true,
                onPressed: () {
                  showDialog<void>(
                    context: context,
                    builder: (_) => PluginLogsDialog(plugin: plugin),
                  );
                },
              ),
              if (plugin.hasCatalog && plugin.enabled)
                FilledButton.icon(
                  key: Key('plugin_browse_${plugin.name}'),
                  style: FilledButton.styleFrom(
                    backgroundColor: c.accentPrimary,
                    foregroundColor: c.onAccent,
                    padding: const EdgeInsets.symmetric(
                      horizontal: AppTokens.space12,
                      vertical: AppTokens.space8,
                    ),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    ),
                  ),
                  onPressed: () => context.go(
                    '/plugins/catalog?plugin=${Uri.encodeComponent(plugin.name)}',
                  ),
                  icon: const Icon(Icons.storefront_outlined, size: 15),
                  label: Text(
                    'Browse Catalog',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: c.onAccent,
                    ),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildServerCard(String? serverUrl) {
    return Consumer(
      builder: (context, ref, _) {
        final queueState = ref.watch(queueProvider);
        final status = queueState.status;
        final libraryState = ref.watch(libraryProvider);
        final resolvedBook = status != null
            ? resolveBookTitle(status.currentBook, libraryState.books)
            : null;
        final progress = status != null
            ? (status.progressPercent / 100.0).clamp(0.0, 1.0)
            : 0.0;
        final c = context.shelfdColors;

        return ShelfdSectionCard(
          icon: Icons.dns_outlined,
          title: 'Server Connection',
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  color: c.accentSecondary,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              Text(
                'Connected',
                style: AppTypography.captionSans(
                  color: c.accentSecondary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Host: ${serverUrl ?? 'Self-hosted API'}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.mono(
                        fontSize: 12,
                        color: c.secondaryInk,
                      ),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  ShelfdSecondaryButton(
                    label: queueState.isLoading
                        ? 'Scanning...'
                        : 'Rescan Library',
                    icon: Icons.sync_rounded,
                    isLoading: queueState.isLoading,
                    compact: true,
                    onPressed: queueState.isLoading
                        ? null
                        : () async {
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content:
                                    Text('Scanning library for new books...'),
                              ),
                            );
                            await ref
                                .read(bookRepositoryProvider)
                                .triggerScan();
                          },
                  ),
                ],
              ),
              if (status != null) ...[
                const SizedBox(height: AppTokens.space16),
                Divider(color: c.subtleBorder, height: 1),
                const SizedBox(height: AppTokens.space16),
                Row(
                  children: [
                    Icon(
                      Icons.auto_awesome_rounded,
                      size: 16,
                      color: c.accentPrimary,
                    ),
                    const SizedBox(width: AppTokens.space8),
                    Expanded(
                      child: Text(
                        'AI Indexing & Vectors',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: c.ink,
                        ),
                      ),
                    ),
                    const SizedBox(width: AppTokens.space8),
                    Text(
                      '${status.progressPercent.toStringAsFixed(0)}%',
                      style: AppTypography.mono(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: c.accentPrimary,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppTokens.space8),
                ClipRRect(
                  borderRadius: BorderRadius.circular(2),
                  child: LinearProgressIndicator(
                    value: progress,
                    minHeight: 4,
                    backgroundColor: c.containerLow,
                    valueColor: AlwaysStoppedAnimation<Color>(c.accentPrimary),
                  ),
                ),
                const SizedBox(height: AppTokens.space8),
                Text(
                  '${status.indexedChapters} of ${status.totalChapters} chapters processed (${status.pendingChapters} pending)',
                  style: AppTypography.captionSans(color: c.mutedInk),
                ),
                if (resolvedBook != null && resolvedBook.isNotEmpty) ...[
                  const SizedBox(height: AppTokens.space4),
                  Text(
                    'Now processing: $resolvedBook',
                    style: AppTypography.captionSans(color: c.ink),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ],
          ),
        );
      },
    );
  }

  Widget _buildAccountCard(ShelfdColors colors, String currentUsername) {
    return ShelfdSectionCard(
      icon: Icons.shield_outlined,
      title: 'Account & Security',
      trailing: StatusBadge(
        label: currentUsername,
        backgroundColor: colors.containerLow,
        textColor: colors.ink,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Default Username Preference
          Text(
            'Default Username',
            style: AppTypography.bodySans(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: colors.ink,
            ),
          ),
          const SizedBox(height: AppTokens.space4),
          Text(
            'Pre-filled when pairing or reconnecting to your server.',
            style: AppTypography.captionSans(color: colors.mutedInk),
          ),
          const SizedBox(height: AppTokens.space8),
          Row(
            children: [
              Expanded(
                child: TextFormField(
                  controller: _defaultUsernameController,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    color: colors.ink,
                  ),
                  decoration: InputDecoration(
                    hintText: 'e.g. admin or your username',
                    isDense: true,
                    prefixIcon: Icon(
                      Icons.person_outline,
                      size: 18,
                      color: colors.mutedInk,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              ShelfdSecondaryButton(
                label: 'Save',
                onPressed: _handleSaveDefaultUsername,
              ),
            ],
          ),
          if (_usernameSavedMessage != null) ...[
            const SizedBox(height: AppTokens.space8),
            Text(
              _usernameSavedMessage!,
              style: AppTypography.captionSans(
                color: colors.accentSecondary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],

          const SizedBox(height: AppTokens.space20),
          Divider(color: colors.subtleBorder, height: 1),
          const SizedBox(height: AppTokens.space12),

          // Progressive Disclosure: Change Password
          Material(
            type: MaterialType.transparency,
            child: InkWell(
              key: const Key('toggle_change_password_button'),
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              onTap: () =>
                  setState(() => _showPasswordForm = !_showPasswordForm),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: AppTokens.space8),
                child: Row(
                  children: [
                    Icon(
                      Icons.lock_reset_outlined,
                      size: 18,
                      color: colors.secondaryInk,
                    ),
                    const SizedBox(width: AppTokens.space8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Change Password',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.bodySans(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                              color: colors.ink,
                            ),
                          ),
                          Text(
                            'Update your account credentials on this server',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.captionSans(
                              color: colors.mutedInk,
                            ),
                          ),
                        ],
                      ),
                    ),
                    Icon(
                      _showPasswordForm
                          ? Icons.keyboard_arrow_up_rounded
                          : Icons.keyboard_arrow_down_rounded,
                      size: 20,
                      color: colors.mutedInk,
                    ),
                  ],
                ),
              ),
            ),
          ),

          if (_showPasswordForm) ...[
            const SizedBox(height: AppTokens.space12),
            if (_passwordError != null) ...[
              ShelfdInlineBanner(
                message: _passwordError!,
                tone: ShelfdBannerTone.error,
              ),
              const SizedBox(height: AppTokens.space12),
            ],
            if (_passwordSuccess != null) ...[
              ShelfdInlineBanner(
                message: _passwordSuccess!,
                tone: ShelfdBannerTone.success,
              ),
              const SizedBox(height: AppTokens.space12),
            ],
            Form(
              key: _passwordFormKey,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  TextFormField(
                    controller: _currentPasswordController,
                    obscureText: _obscureCurrent,
                    style: AppTypography.bodySans(
                      fontSize: 14,
                      color: colors.ink,
                    ),
                    decoration: InputDecoration(
                      labelText: 'Current Password',
                      hintText: '••••••••',
                      isDense: true,
                      prefixIcon: Icon(
                        Icons.lock_outline,
                        size: 18,
                        color: colors.mutedInk,
                      ),
                      suffixIcon: IconButton(
                        icon: Icon(
                          _obscureCurrent
                              ? Icons.visibility_off
                              : Icons.visibility,
                          size: 18,
                          color: colors.mutedInk,
                        ),
                        onPressed: () =>
                            setState(() => _obscureCurrent = !_obscureCurrent),
                      ),
                    ),
                    validator: (v) => (v == null || v.isEmpty)
                        ? 'Current password required'
                        : null,
                  ),
                  const SizedBox(height: AppTokens.space12),
                  TextFormField(
                    controller: _newPasswordController,
                    obscureText: _obscureNew,
                    style: AppTypography.bodySans(
                      fontSize: 14,
                      color: colors.ink,
                    ),
                    decoration: InputDecoration(
                      labelText: 'New Password',
                      hintText: '••••••••',
                      helperText: 'Minimum 6 characters',
                      isDense: true,
                      prefixIcon: Icon(
                        Icons.lock_reset_outlined,
                        size: 18,
                        color: colors.mutedInk,
                      ),
                      suffixIcon: IconButton(
                        icon: Icon(
                          _obscureNew ? Icons.visibility_off : Icons.visibility,
                          size: 18,
                          color: colors.mutedInk,
                        ),
                        onPressed: () =>
                            setState(() => _obscureNew = !_obscureNew),
                      ),
                    ),
                    validator: (v) {
                      if (v == null || v.isEmpty) {
                        return 'New password required';
                      }
                      if (v.length < 6) {
                        return 'Password must be at least 6 characters';
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: AppTokens.space12),
                  TextFormField(
                    controller: _confirmPasswordController,
                    obscureText: _obscureConfirm,
                    style: AppTypography.bodySans(
                      fontSize: 14,
                      color: colors.ink,
                    ),
                    decoration: InputDecoration(
                      labelText: 'Confirm New Password',
                      hintText: '••••••••',
                      isDense: true,
                      prefixIcon: Icon(
                        Icons.check_circle_outline,
                        size: 18,
                        color: colors.mutedInk,
                      ),
                      suffixIcon: IconButton(
                        icon: Icon(
                          _obscureConfirm
                              ? Icons.visibility_off
                              : Icons.visibility,
                          size: 18,
                          color: colors.mutedInk,
                        ),
                        onPressed: () =>
                            setState(() => _obscureConfirm = !_obscureConfirm),
                      ),
                    ),
                    validator: (v) {
                      if (v == null || v.isEmpty) {
                        return 'Please confirm new password';
                      }
                      if (v != _newPasswordController.text) {
                        return 'Passwords do not match';
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: AppTokens.space16),
                  PrimaryButton(
                    label: 'Update Password',
                    icon: Icons.check_rounded,
                    isLoading: _isChangingPassword,
                    onPressed: _handleChangePassword,
                  ),
                ],
              ),
            ),
          ],

          const SizedBox(height: AppTokens.space20),
          Divider(color: colors.subtleBorder, height: 1),
          const SizedBox(height: AppTokens.space16),

          // Disconnect / Sign Out cleanly anchored at the bottom of Account
          ShelfdDangerButton(
            label: 'Disconnect / Sign Out',
            icon: Icons.logout_rounded,
            onPressed: _handleLogout,
          ),
        ],
      ),
    );
  }
}
