import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/utils/connect_diagnostics.dart';
import '../../../data/utils/url_normalizer.dart';
import '../../../platform/qr_scanner_service.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';

class ConnectView extends ConsumerStatefulWidget {
  const ConnectView({super.key});

  @override
  ConsumerState<ConnectView> createState() => _ConnectViewState();
}

class _ConnectViewState extends ConsumerState<ConnectView> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _urlController;
  late final TextEditingController _userController;
  late final TextEditingController _passwordController;
  bool _obscurePassword = true;
  ConnectDiagnostic? _localDiagnostic;

  @override
  void initState() {
    super.initState();
    final storage = ref.read(storageServiceProvider);
    final savedUrl = storage.getServerUrl();
    final initialUrl = (savedUrl != null && savedUrl.isNotEmpty)
        ? savedUrl
        : resolveDefaultServerUrl();
    _urlController = TextEditingController(text: initialUrl);
    final savedUser = storage.getSavedUsername();
    _userController = TextEditingController(
      text: savedUser != null && savedUser.isNotEmpty ? savedUser : 'admin',
    );
    _passwordController = TextEditingController();
  }

  @override
  void dispose() {
    _urlController.dispose();
    _userController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _navigateAfterAuth() {
    final router = GoRouter.maybeOf(context);
    if (router == null) return;
    String? rawRedirect;
    try {
      rawRedirect = GoRouterState.of(context).uri.queryParameters['redirect'];
    } catch (_) {
      rawRedirect = Uri.base.queryParameters['redirect'];
    }
    final target = UrlNormalizer.sanitizeSameOriginRedirect(rawRedirect) ?? '/books';
    router.go(target);
  }

  Future<void> _onScanQrPressed() async {
    setState(() {
      _localDiagnostic = null;
    });
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      try {
        final raw = await ref.read(qrScannerServiceProvider).scanQrCode();
        if (!mounted || raw == null || raw.isEmpty) {
          return;
        }
        final ok = await ref.read(authProvider.notifier).pairWithQrUri(raw);
        if (ok && mounted) {
          _navigateAfterAuth();
        }
        return;
      } on QrScannerUnavailableException {
        if (mounted) {
          await _showPastePairingDialog();
        }
        return;
      } catch (e) {
        if (mounted) {
          setState(() {
            _localDiagnostic = ConnectDiagnostics.fromError(e);
          });
        }
        return;
      }
    }
    await _showPastePairingDialog();
  }

  Future<void> _showPastePairingDialog() async {
    final raw = await showDialog<String>(
      context: context,
      builder: (_) => const _PastePairingDialog(),
    );
    if (raw != null && raw.isNotEmpty && mounted) {
      final ok = await ref.read(authProvider.notifier).pairWithQrUri(raw);
      if (ok && mounted) {
        _navigateAfterAuth();
      }
    }
  }

  Future<bool> _confirmCleartextConnection(String targetUrl) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogColors = ctx.shelfdColors;
        return AlertDialog(
          backgroundColor: dialogColors.surface,
          title: Text(
            'Unencrypted HTTP Connection',
            style: AppTypography.titleSerif(
              fontSize: 18,
              color: dialogColors.ink,
            ),
          ),
          content: Text(
            'The server address $targetUrl uses unencrypted http:// over a non-private network. Your password and device token could be intercepted in transit. Prefer https:// or Tailscale unless you trust this network.',
            style: AppTypography.bodySans(
              fontSize: 13,
              color: dialogColors.secondaryInk,
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              key: const Key('confirm_cleartext_http_button'),
              style: ElevatedButton.styleFrom(
                backgroundColor: dialogColors.accentPrimary,
                foregroundColor: dialogColors.onAccent,
              ),
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Connect Over HTTP'),
            ),
          ],
        );
      },
    );
    return confirmed == true;
  }

  Future<void> _handleConnect() async {
    setState(() {
      _localDiagnostic = null;
    });
    if (!_formKey.currentState!.validate()) return;

    final rawInput = _urlController.text.trim().isNotEmpty
        ? _urlController.text.trim()
        : resolveDefaultServerUrl();

    NormalizedServerUrl normalized;
    try {
      normalized = UrlNormalizer.normalizeServerUrl(rawInput);
    } on FormatException catch (e) {
      setState(() {
        _localDiagnostic = ConnectDiagnostics.fromError(e, serverUrl: rawInput);
      });
      return;
    }

    if (!kIsWeb && normalized.requiresCleartextWarning) {
      final proceed = await _confirmCleartextConnection(normalized.url);
      if (!proceed || !mounted) return;
    }

    _urlController.text = normalized.url;
    final user = _userController.text.trim();
    final pass = _passwordController.text;

    final success = await ref
        .read(authProvider.notifier)
        .login(normalized.url, user, pass);
    if (success && mounted) {
      TextInput.finishAutofillContext();
      _navigateAfterAuth();
    }
  }

  Future<void> _copyDiagnostics(ConnectDiagnostic diag) async {
    await Clipboard.setData(ClipboardData(text: diag.clipboardDetails));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Connection diagnostics copied to clipboard.'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final horizontalPad = Responsive.horizontalPadding(context);
    final colors = context.shelfdColors;
    final activeDiagnostic = _localDiagnostic ?? authState.diagnostic;
    final errorMessage = _localDiagnostic?.message ?? authState.error;

    return Scaffold(
      backgroundColor: colors.background,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 540),
            child: SingleChildScrollView(
              padding: EdgeInsets.symmetric(
                horizontal: horizontalPad,
                vertical: AppTokens.space24,
              ),
              child: AutofillGroup(
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Center(
                        child: Container(
                          width: 56,
                          height: 56,
                          decoration: BoxDecoration(
                            color: colors.accentPrimary,
                            borderRadius: BorderRadius.circular(AppTokens.radiusLg),
                            boxShadow: [
                              BoxShadow(
                                color: colors.cardShadowColor,
                                blurRadius: 16,
                                offset: const Offset(0, 6),
                              ),
                            ],
                          ),
                          child: Icon(
                            Icons.bookmark_outline_rounded,
                            color: colors.onAccent,
                            size: 30,
                          ),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space16),
                      Text(
                        'Connect to your Library',
                        textAlign: TextAlign.center,
                        style: AppTypography.displaySerif(
                          fontSize: 28,
                          fontWeight: FontWeight.w600,
                          color: colors.ink,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        'Pair with your self-hosted Shelfd homelab daemon or Audiobookshelf companion server',
                        textAlign: TextAlign.center,
                        style: AppTypography.bodySans(
                          fontSize: 14,
                          color: colors.secondaryInk,
                        ),
                      ),
                      const SizedBox(height: AppTokens.space32),
                      BentoCard(
                        key: const Key('scan_server_qr_card'),
                        onTap: authState.isLoading ? null : _onScanQrPressed,
                        child: Column(
                          children: [
                            Container(
                              width: 56,
                              height: 56,
                              decoration: BoxDecoration(
                                color: colors.accentContainer,
                                borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                              ),
                              child: Icon(
                                Icons.qr_code_scanner_rounded,
                                size: 30,
                                color: colors.accentPrimary,
                              ),
                            ),
                            const SizedBox(height: AppTokens.space12),
                            Text(
                              'Scan Server QR Code',
                              style: AppTypography.titleSerif(
                                fontSize: 18,
                                color: colors.ink,
                              ),
                            ),
                            const SizedBox(height: AppTokens.space4),
                            Text(
                              'Instant permissionless pairing from Web Settings',
                              style: AppTypography.bodySans(
                                fontSize: 13,
                                color: colors.secondaryInk,
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Center(
                        child: TextButton.icon(
                          key: const Key('paste_pairing_link_button'),
                          onPressed: authState.isLoading
                              ? null
                              : _showPastePairingDialog,
                          icon: Icon(
                            Icons.content_paste_rounded,
                            size: 16,
                            color: colors.secondaryInk,
                          ),
                          label: Text(
                            'Paste Pairing Link',
                            style: AppTypography.captionSans(
                              color: colors.secondaryInk,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space16),
                      Row(
                        children: [
                          Expanded(child: Divider(color: colors.border)),
                          Padding(
                            padding: const EdgeInsets.symmetric(
                              horizontal: AppTokens.space12,
                            ),
                            child: Text(
                              'Or connect manually',
                              style: AppTypography.labelCaps(
                                fontSize: 11,
                                color: colors.mutedInk,
                              ),
                            ),
                          ),
                          Expanded(child: Divider(color: colors.border)),
                        ],
                      ),
                      const SizedBox(height: AppTokens.space24),
                      if (errorMessage != null) ...[
                        Container(
                          padding: const EdgeInsets.all(AppTokens.space12),
                          decoration: BoxDecoration(
                            color: colors.errorBg,
                            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                            border: Border.all(color: colors.errorBorder),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                errorMessage,
                                style: AppTypography.bodySans(
                                  fontSize: 13,
                                  color: colors.errorText,
                                ),
                              ),
                              if (activeDiagnostic != null) ...[
                                const SizedBox(height: AppTokens.space8),
                                Align(
                                  alignment: Alignment.centerRight,
                                  child: TextButton.icon(
                                    key: const Key('copy_diagnostics_button'),
                                    style: TextButton.styleFrom(
                                      foregroundColor: colors.errorText,
                                      visualDensity: VisualDensity.compact,
                                    ),
                                    onPressed: () =>
                                        _copyDiagnostics(activeDiagnostic),
                                    icon: const Icon(
                                      Icons.copy_all_rounded,
                                      size: 15,
                                    ),
                                    label: const Text(
                                      'Copy Connection Diagnostics',
                                    ),
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ),
                        const SizedBox(height: AppTokens.space16),
                      ],
                      TextFormField(
                        controller: _urlController,
                        keyboardType: TextInputType.url,
                        autofillHints: const [AutofillHints.url],
                        style: AppTypography.mono(fontSize: 13, color: colors.ink),
                        decoration: InputDecoration(
                          labelText: 'Server URL',
                          hintText: 'shelfd.example.com or 192.168.1.50:8080',
                          helperText: kIsWeb
                              ? 'Connected to self-hosted API origin'
                              : null,
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
                            borderSide: BorderSide(
                              color: colors.accentPrimary,
                              width: 1.5,
                            ),
                          ),
                          prefixIcon: Icon(
                            Icons.lan_outlined,
                            color: colors.mutedInk,
                          ),
                        ),
                        validator: (v) =>
                            (v == null || v.trim().isEmpty) ? 'Server URL required' : null,
                      ),
                      const SizedBox(height: AppTokens.space16),
                      TextFormField(
                        controller: _userController,
                        autofillHints: const [AutofillHints.username],
                        style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                        decoration: InputDecoration(
                          labelText: 'Username',
                          hintText: 'admin',
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
                            borderSide: BorderSide(
                              color: colors.accentPrimary,
                              width: 1.5,
                            ),
                          ),
                          prefixIcon: Icon(
                            Icons.person_outline,
                            color: colors.mutedInk,
                          ),
                        ),
                        validator: (v) =>
                            (v == null || v.trim().isEmpty) ? 'Username required' : null,
                      ),
                      const SizedBox(height: AppTokens.space16),
                      TextFormField(
                        controller: _passwordController,
                        obscureText: _obscurePassword,
                        autofillHints: const [AutofillHints.password],
                        onFieldSubmitted: (_) => _handleConnect(),
                        style: AppTypography.bodySans(fontSize: 14, color: colors.ink),
                        decoration: InputDecoration(
                          labelText: 'Password',
                          hintText: '••••••••',
                          helperText: kIsWeb
                              ? 'Check container logs on first startup for generated password'
                              : null,
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
                            borderSide: BorderSide(
                              color: colors.accentPrimary,
                              width: 1.5,
                            ),
                          ),
                          prefixIcon: Icon(
                            Icons.lock_outline,
                            color: colors.mutedInk,
                          ),
                          suffixIcon: IconButton(
                            icon: Icon(
                              _obscurePassword
                                  ? Icons.visibility_off
                                  : Icons.visibility,
                              color: colors.mutedInk,
                            ),
                            onPressed: () => setState(
                              () => _obscurePassword = !_obscurePassword,
                            ),
                          ),
                        ),
                        validator: (v) =>
                            (v == null || v.isEmpty) ? 'Password required' : null,
                      ),
                      const SizedBox(height: AppTokens.space24),
                      PrimaryButton(
                        label: 'Connect & Sync Library',
                        isLoading: authState.isLoading,
                        onPressed: authState.isLoading ? null : _handleConnect,
                      ),
                      const SizedBox(height: AppTokens.space32),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(
                            Icons.shield_outlined,
                            size: 16,
                            color: colors.mutedInk,
                          ),
                          const SizedBox(height: AppTokens.space8),
                          Flexible(
                            child: Text(
                              'Direct connection to your private server. No third-party cloud.',
                              style: AppTypography.captionSans(
                                fontSize: 12,
                                color: colors.mutedInk,
                              ),
                              textAlign: TextAlign.center,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _PastePairingDialog extends StatefulWidget {
  const _PastePairingDialog();

  @override
  State<_PastePairingDialog> createState() => _PastePairingDialogState();
}

class _PastePairingDialogState extends State<_PastePairingDialog> {
  final TextEditingController _pasteController = TextEditingController();
  String? _dialogError;

  @override
  void dispose() {
    _pasteController.dispose();
    super.dispose();
  }

  void _submit() {
    final raw = _pasteController.text.trim();
    if (raw.isEmpty) {
      setState(() {
        _dialogError = 'Paste a shelfd://pair link first.';
      });
      return;
    }
    Navigator.of(context).pop(raw);
  }

  @override
  Widget build(BuildContext context) {
    final dialogColors = context.shelfdColors;
    return AlertDialog(
      backgroundColor: dialogColors.surface,
      title: Text(
        'Pair with Server Link',
        style: AppTypography.titleSerif(
          fontSize: 18,
          color: dialogColors.ink,
        ),
      ),
      content: SizedBox(
        width: 420,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Open Settings on your Shelfd web dashboard, click Pair Mobile Device, and copy the shelfd://pair link.',
                style: AppTypography.bodySans(
                  fontSize: 13,
                  color: dialogColors.secondaryInk,
                ),
              ),
              const SizedBox(height: AppTokens.space12),
              TextField(
                key: const Key('pairing_link_input'),
                controller: _pasteController,
                onSubmitted: (_) => _submit(),
                style: AppTypography.mono(
                  fontSize: 12,
                  color: dialogColors.ink,
                ),
                decoration: InputDecoration(
                  labelText: 'Pairing Link',
                  hintText: 'shelfd://pair?u=https://...&c=...',
                  errorText: _dialogError,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          style: TextButton.styleFrom(
            foregroundColor: dialogColors.secondaryInk,
          ),
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        ElevatedButton(
          key: const Key('pair_device_submit_button'),
          style: ElevatedButton.styleFrom(
            backgroundColor: dialogColors.accentPrimary,
            foregroundColor: dialogColors.onAccent,
          ),
          onPressed: _submit,
          child: const Text('Pair Device'),
        ),
      ],
    );
  }
}

