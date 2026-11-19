import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/plugin.dart';
import '../../core/app_shell.dart';
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
      final callbackPlugin = Uri.base.queryParameters['plugin']?.trim() ??
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
    if (index == 0) {
      context.go('/books');
    } else if (index == 1) {
      context.go('/series');
    } else if (index == 2) {
      context.go('/authors');
    } else if (index == 3) {
      context.go('/search');
    } else if (index == 4) {
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
      await ref.read(authProvider.notifier).changePassword(currentPass, newPass);
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
            backgroundColor: Color(0xFF2B8A3E),
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
            _passwordError = colon != -1 ? msg.substring(colon + 1).trim() : msg;
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

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final isDesktop = Responsive.isDesktop(context);
    final currentUsername = authState.user?.username ?? 'Connected User';
    final colors = context.shelfdColors;

    final hasAppShell = context.findAncestorWidgetOfExactType<AppShell>() != null;

    final topBar = isDesktop
        ? null
        : ShelfdTopBar(
            title: 'Settings',
            subtitle: 'Homelab Daemon Configuration',
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
              Text(
                'Settings',
                style: AppTypography.displaySerif(
                  fontSize: 28,
                  fontWeight: FontWeight.w600,
                  color: colors.ink,
                ),
              ),
              const SizedBox(height: AppTokens.space4),
              Text(
                'Daemon configuration, security & account',
                style: AppTypography.bodySans(fontSize: 14, color: colors.secondaryInk),
              ),
              const SizedBox(height: AppTokens.space24),
            ],

            // Appearance & Theme BentoCard
            Consumer(
              builder: (context, ref, _) {
                final readerSettings = ref.watch(readerSettingsProvider);
                final c = context.shelfdColors;
                return BentoCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.palette_outlined, size: 20, color: c.accentPrimary),
                          const SizedBox(width: AppTokens.space8),
                          Text(
                            'Appearance & Theme',
                            style: AppTypography.titleSerif(fontSize: 18, color: c.ink),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        'Choose the interface and reading color palette across the application.',
                        style: AppTypography.captionSans(color: c.secondaryInk),
                      ),
                      const SizedBox(height: AppTokens.space16),
                      SizedBox(
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
                              ref.read(readerSettingsProvider.notifier).setThemeMode(set.first);
                            }
                          },
                        ),
                      ),
                    ],
                  ),
                );
              },
            ),

            const SizedBox(height: AppTokens.space20),

            // Account & Security BentoCard
            BentoCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(Icons.shield_outlined, size: 20, color: colors.accentPrimary),
                      const SizedBox(width: AppTokens.space8),
                      Text(
                        'Account & Security',
                        style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
                      ),
                      const Spacer(),
                      StatusBadge(
                        label: currentUsername,
                        backgroundColor: colors.containerLow,
                        textColor: colors.ink,
                      ),
                    ],
                  ),
                  const SizedBox(height: AppTokens.space16),
                  Divider(color: colors.subtleBorder, height: 1),
                  const SizedBox(height: AppTokens.space16),

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
                    'Preferred username pre-filled during pairing and server connection.',
                    style: AppTypography.captionSans(color: colors.mutedInk),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  Row(
                    children: [
                      Expanded(
                        child: TextFormField(
                          controller: _defaultUsernameController,
                          style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                          decoration: InputDecoration(
                            hintText: 'e.g. admin or your username',
                            isDense: true,
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                            ),
                            prefixIcon: Icon(Icons.person_outline, size: 20, color: colors.mutedInk),
                          ),
                        ),
                      ),
                      const SizedBox(width: AppTokens.space8),
                      OutlinedButton(
                        style: OutlinedButton.styleFrom(
                          foregroundColor: colors.ink,
                          side: BorderSide(color: colors.border),
                          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                        ),
                        onPressed: _handleSaveDefaultUsername,
                        child: const Text('Save'),
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

                  const SizedBox(height: AppTokens.space24),
                  Divider(color: colors.subtleBorder, height: 1),
                  const SizedBox(height: AppTokens.space16),

                  // Change Password Sub-section
                  Text(
                    'Change Password',
                    style: AppTypography.bodySans(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: colors.ink,
                    ),
                  ),
                  const SizedBox(height: AppTokens.space4),
                  Text(
                    'Update your account credentials on the homelab daemon.',
                    style: AppTypography.captionSans(color: colors.mutedInk),
                  ),
                  const SizedBox(height: AppTokens.space16),

                  if (_passwordError != null) ...[
                    Container(
                      padding: const EdgeInsets.all(AppTokens.space12),
                      decoration: BoxDecoration(
                        color: colors.errorBg,
                        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                        border: Border.all(color: colors.errorBorder),
                      ),
                      child: Row(
                        children: [
                          Icon(Icons.error_outline, size: 18, color: colors.errorText),
                          const SizedBox(width: AppTokens.space8),
                          Expanded(
                            child: Text(
                              _passwordError!,
                              style: AppTypography.bodySans(color: colors.errorText, fontSize: 13),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  if (_passwordSuccess != null) ...[
                    Container(
                      padding: const EdgeInsets.all(AppTokens.space12),
                      decoration: BoxDecoration(
                        color: colors.isDark
                            ? colors.accentSecondaryContainer
                            : AppTokens.completedBadgeBg,
                        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                        border: Border.all(color: colors.accentSecondary.withValues(alpha: 0.4)),
                      ),
                      child: Row(
                        children: [
                          Icon(Icons.check_circle_outline, size: 18, color: colors.accentSecondary),
                          const SizedBox(width: AppTokens.space8),
                          Expanded(
                            child: Text(
                              _passwordSuccess!,
                              style: AppTypography.bodySans(color: colors.accentSecondary, fontSize: 13),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: AppTokens.space16),
                  ],

                  Form(
                    key: _passwordFormKey,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        TextFormField(
                          controller: _currentPasswordController,
                          obscureText: _obscureCurrent,
                          style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                          decoration: InputDecoration(
                            labelText: 'Current Password',
                            hintText: '••••••••',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                            ),
                            prefixIcon: Icon(Icons.lock_outline, color: colors.mutedInk),
                            suffixIcon: IconButton(
                              icon: Icon(
                                _obscureCurrent ? Icons.visibility_off : Icons.visibility,
                                color: colors.mutedInk,
                              ),
                              onPressed: () => setState(() => _obscureCurrent = !_obscureCurrent),
                            ),
                          ),
                          validator: (v) => (v == null || v.isEmpty) ? 'Current password required' : null,
                        ),
                        const SizedBox(height: AppTokens.space16),

                        TextFormField(
                          controller: _newPasswordController,
                          obscureText: _obscureNew,
                          style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                          decoration: InputDecoration(
                            labelText: 'New Password',
                            hintText: '••••••••',
                            helperText: 'Minimum 6 characters',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                            ),
                            prefixIcon: Icon(Icons.lock_reset_outlined, color: colors.mutedInk),
                            suffixIcon: IconButton(
                              icon: Icon(
                                _obscureNew ? Icons.visibility_off : Icons.visibility,
                                color: colors.mutedInk,
                              ),
                              onPressed: () => setState(() => _obscureNew = !_obscureNew),
                            ),
                          ),
                          validator: (v) {
                            if (v == null || v.isEmpty) return 'New password required';
                            if (v.length < 6) return 'Password must be at least 6 characters';
                            return null;
                          },
                        ),
                        const SizedBox(height: AppTokens.space16),

                        TextFormField(
                          controller: _confirmPasswordController,
                          obscureText: _obscureConfirm,
                          style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                          decoration: InputDecoration(
                            labelText: 'Confirm New Password',
                            hintText: '••••••••',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.border),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
                            ),
                            prefixIcon: Icon(Icons.check_circle_outline, color: colors.mutedInk),
                            suffixIcon: IconButton(
                              icon: Icon(
                                _obscureConfirm ? Icons.visibility_off : Icons.visibility,
                                color: colors.mutedInk,
                              ),
                              onPressed: () => setState(() => _obscureConfirm = !_obscureConfirm),
                            ),
                          ),
                          validator: (v) {
                            if (v == null || v.isEmpty) return 'Please confirm new password';
                            if (v != _newPasswordController.text) return 'Passwords do not match';
                            return null;
                          },
                        ),
                        const SizedBox(height: AppTokens.space20),

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
              ),
            ),

            const SizedBox(height: AppTokens.space20),

            // Uploads & Ingestion Card
            Consumer(
              builder: (context, ref, _) {
                final uploadState = ref.watch(uploadProvider);
                final c = context.shelfdColors;
                return BentoCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.cloud_upload_outlined, size: 20, color: c.accentPrimary),
                          const SizedBox(width: AppTokens.space8),
                          Text(
                            'Uploads & Ingestion',
                            style: AppTypography.titleSerif(fontSize: 18, color: c.ink),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppTokens.space12),
                      SwitchListTile.adaptive(
                        contentPadding: EdgeInsets.zero,
                        title: Text(
                          'Auto-commit uploads without review',
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: c.ink,
                          ),
                        ),
                        subtitle: Text(
                          'Automatically ingest dropped EPUBs into /library using extracted metadata without requiring manual approval.',
                          style: AppTypography.captionSans(color: c.mutedInk),
                        ),
                        value: uploadState.autoCommit,
                        activeTrackColor: c.accentPrimary,
                        onChanged: (val) => ref.read(uploadProvider.notifier).toggleAutoCommit(val),
                      ),
                      const SizedBox(height: AppTokens.space12),
                      OutlinedButton.icon(
                        style: OutlinedButton.styleFrom(
                          foregroundColor: c.ink,
                          side: BorderSide(color: c.border),
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppTokens.space16,
                            vertical: AppTokens.space12,
                          ),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          ),
                        ),
                        icon: const Icon(Icons.cloud_upload_outlined, size: 18),
                        label: const Text('Open Uploads & Staged Queue'),
                        onPressed: () => context.go('/uploads'),
                      ),
                    ],
                  ),
                );
              },
            ),

            const SizedBox(height: AppTokens.space20),

            // Library Utilities Card
            BentoCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(Icons.handyman_outlined, size: 20, color: colors.accentPrimary),
                      const SizedBox(width: AppTokens.space8),
                      Text(
                        'Library Utilities',
                        style: AppTypography.titleSerif(fontSize: 18, color: colors.ink),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppTokens.space8),
                  Text(
                    'Detect duplicate books, merge book editions, and resolve library conflicts.',
                    style: AppTypography.captionSans(color: colors.mutedInk),
                  ),
                  const SizedBox(height: AppTokens.space12),
                  OutlinedButton.icon(
                    style: OutlinedButton.styleFrom(
                      foregroundColor: colors.ink,
                      side: BorderSide(color: colors.border),
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppTokens.space16,
                        vertical: AppTokens.space12,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                      ),
                    ),
                    icon: const Icon(Icons.merge_type_rounded, size: 18),
                    label: const Text('Manage Duplicates & Book Merge'),
                    onPressed: () => context.go('/utilities'),
                  ),
                ],
              ),
            ),

            const SizedBox(height: AppTokens.space20),

            // Server & Live Indexing Status Card
            Consumer(
              builder: (context, ref, _) {
                final queueState = ref.watch(queueProvider);
                final status = queueState.status;
                final libraryState = ref.watch(libraryProvider);
                final resolvedBook =
                    status != null ? resolveBookTitle(status.currentBook, libraryState.books) : null;
                final progress = status != null ? (status.progressPercent / 100.0).clamp(0.0, 1.0) : 0.0;
                final c = context.shelfdColors;

                return BentoCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.dns_outlined, size: 20, color: c.accentPrimary),
                          const SizedBox(width: AppTokens.space8),
                          Text(
                            'Server Connection',
                            style: AppTypography.titleSerif(fontSize: 18, color: c.ink),
                          ),
                          const Spacer(),
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
                            style: AppTypography.captionSans(color: c.accentSecondary),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        'Host: ${authState.serverUrl ?? 'Self-hosted API'}',
                        style: AppTypography.mono(fontSize: 12, color: c.secondaryInk),
                      ),

                      if (status != null) ...[
                        const SizedBox(height: AppTokens.space16),
                        Divider(color: c.subtleBorder, height: 1),
                        const SizedBox(height: AppTokens.space16),
                        Row(
                          children: [
                            Icon(Icons.auto_awesome_rounded, size: 16, color: c.accentPrimary),
                            const SizedBox(width: AppTokens.space8),
                            Text(
                              'AI Indexing & Vectors',
                              style: AppTypography.bodySans(
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                                color: c.ink,
                              ),
                            ),
                            const Spacer(),
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
                          const SizedBox(height: 4),
                          Text(
                            'Now processing: $resolvedBook',
                            style: AppTypography.captionSans(color: c.ink),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ],
                      const SizedBox(height: AppTokens.space16),
                      OutlinedButton.icon(
                        style: OutlinedButton.styleFrom(
                          foregroundColor: c.ink,
                          side: BorderSide(color: c.border),
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppTokens.space16,
                            vertical: AppTokens.space12,
                          ),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          ),
                        ),
                        icon: queueState.isLoading
                            ? SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                  color: c.accentPrimary,
                                ),
                              )
                            : const Icon(Icons.sync_rounded, size: 18),
                        label: Text(queueState.isLoading ? 'Scanning...' : 'Rescan Library for New Books'),
                        onPressed: queueState.isLoading
                            ? null
                            : () async {
                                ScaffoldMessenger.of(context).showSnackBar(
                                  const SnackBar(content: Text('Scanning library for new books...')),
                                );
                                await ref.read(bookRepositoryProvider).triggerScan();
                              },
                      ),
                    ],
                  ),
                );
              },
            ),

            const SizedBox(height: AppTokens.space24),

            // Disconnect / Sign Out Button
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: colors.errorText,
                side: BorderSide(color: colors.errorBorder),
                backgroundColor: colors.errorBg.withValues(alpha: 0.35),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
              ),
              icon: const Icon(Icons.logout_rounded, size: 18),
              label: const Text('Disconnect / Sign Out'),
              onPressed: _handleLogout,
            ),

            const SizedBox(height: AppTokens.space24),

            // Plugins & Storefronts Card
            Consumer(
              builder: (context, ref, _) {
                final pluginsState = ref.watch(pluginsProvider);
                final c = context.shelfdColors;
                return _buildPluginsSettingsCard(context, c, pluginsState);
              },
            ),
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

  Widget _buildPluginsSettingsCard(
    BuildContext context,
    ShelfdColors c,
    PluginsState pluginsState,
  ) {
    return BentoCard(
      key: const Key('plugins_settings_card'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.extension_outlined, size: 20, color: c.accentPrimary),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Plugins & Storefronts',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(fontSize: 18, color: c.ink),
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              OutlinedButton.icon(
                key: const Key('plugins_rescan_button'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: c.ink,
                  side: BorderSide(color: c.border),
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space12,
                    vertical: AppTokens.space8,
                  ),
                ),
                onPressed: pluginsState.isLoading
                    ? null
                    : () => ref.read(pluginsProvider.notifier).discover(),
                icon: const Icon(Icons.refresh_rounded, size: 16),
                label: const Text('Rescan'),
              ),
              const SizedBox(width: AppTokens.space8),
              FilledButton.icon(
                key: const Key('plugins_install_zip_button'),
                style: FilledButton.styleFrom(
                  backgroundColor: c.accentPrimary,
                  foregroundColor: c.onAccent,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space12,
                    vertical: AppTokens.space8,
                  ),
                ),
                onPressed:
                    pluginsState.isInstalling ? null : _handleInstallPluginZip,
                icon: const Icon(Icons.upload_file_rounded, size: 16),
                label: Text(
                  pluginsState.isInstalling ? 'Installing...' : 'Install .zip',
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space8),
          Text(
            'Install runtime plugins to browse external book storefronts, import bundles into the staged review queue, or expose MCP tools.',
            style: AppTypography.captionSans(color: c.mutedInk),
          ),
          const SizedBox(height: AppTokens.space16),
          if (pluginsState.errorMessage != null) ...[
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: c.errorBg,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                border: Border.all(color: c.errorBorder),
              ),
              child: Text(
                pluginsState.errorMessage!,
                style: AppTypography.bodySans(fontSize: 13, color: c.errorText),
              ),
            ),
            const SizedBox(height: AppTokens.space12),
          ],
          if (pluginsState.successMessage != null) ...[
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppTokens.space12),
              decoration: BoxDecoration(
                color: c.accentSecondaryContainer,
                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              ),
              child: Text(
                pluginsState.successMessage!,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  color: c.accentSecondary,
                ),
              ),
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
                    : 'No plugins installed yet. Upload a .zip plugin archive or place plugins into your server plugins directory.',
                style: AppTypography.bodySans(fontSize: 13, color: c.mutedInk),
              ),
            )
          else
            ...pluginsState.plugins.map((plugin) {
              return Padding(
                padding: const EdgeInsets.only(bottom: AppTokens.space12),
                child: _buildPluginRowCard(context, c, plugin),
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
  ) {
    final isHealthy =
        plugin.status == 'running' || plugin.status == 'idle';
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
                activeTrackColor: c.accentPrimary,
                onChanged: (val) => ref
                    .read(pluginsProvider.notifier)
                    .setEnabled(plugin.name, val),
              ),
            ],
          ),
          if (plugin.description != null &&
              plugin.description!.trim().isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              plugin.description!,
              style: AppTypography.captionSans(color: c.secondaryInk),
            ),
          ],
          if (plugin.permissions.networkDomains.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              'Domains: ${plugin.permissions.networkDomains.join(', ')}',
              style: AppTypography.mono(fontSize: 11, color: c.mutedInk),
            ),
          ],
          if (plugin.lastError != null && plugin.lastError!.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              plugin.lastError!,
              style: AppTypography.mono(fontSize: 11, color: c.errorText),
            ),
          ],
          const SizedBox(height: AppTokens.space8),
          Wrap(
            spacing: AppTokens.space8,
            runSpacing: AppTokens.space8,
            children: [
              if (plugin.hasConfigSchema ||
                  plugin.commands.isNotEmpty ||
                  plugin.hasAuth)
                OutlinedButton.icon(
                  key: Key('plugin_configure_${plugin.name}'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: c.ink,
                    side: BorderSide(color: c.border),
                  ),
                  onPressed: () {
                    showDialog<bool>(
                      context: context,
                      builder: (_) => PluginConfigDialog(plugin: plugin),
                    );
                  },
                  icon: const Icon(Icons.tune_rounded, size: 15),
                  label: Text(
                    !plugin.configured ? 'Configure (Required)' : 'Configure',
                  ),
                ),
              OutlinedButton.icon(
                key: Key('plugin_logs_${plugin.name}'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: c.ink,
                  side: BorderSide(color: c.border),
                ),
                onPressed: () {
                  showDialog<void>(
                    context: context,
                    builder: (_) => PluginLogsDialog(plugin: plugin),
                  );
                },
                icon: const Icon(Icons.terminal_rounded, size: 15),
                label: const Text('Logs'),
              ),
              if (plugin.hasCatalog && plugin.enabled)
                FilledButton.icon(
                  key: Key('plugin_browse_${plugin.name}'),
                  style: FilledButton.styleFrom(
                    backgroundColor: c.accentPrimary,
                    foregroundColor: c.onAccent,
                  ),
                  onPressed: () => context.go(
                    '/plugins/catalog?plugin=${Uri.encodeComponent(plugin.name)}',
                  ),
                  icon: const Icon(Icons.storefront_outlined, size: 15),
                  label: const Text('Browse Catalog'),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
