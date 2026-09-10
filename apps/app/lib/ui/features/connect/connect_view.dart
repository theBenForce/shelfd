import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../core/responsive.dart';
import '../../core/shared_layout.dart';
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

  @override
  void initState() {
    super.initState();
    final storage = ref.read(storageServiceProvider);
    final savedUrl = storage.getServerUrl();
    final initialUrl = (savedUrl != null && savedUrl.isNotEmpty) ? savedUrl : resolveDefaultServerUrl();
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

  void _onScanQrPressed() {
    // In mobile/desktop client, if QR camera is not configured or simulated,
    // show a dialog allowing immediate QR paste/simulation
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Scan Server QR Code', style: AppTypography.titleSerif(fontSize: 18)),
        content: const Text(
          'Camera scanner active or paste your pairing connect URL from the Shelfd web dashboard.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppTokens.charcoalInk,
              foregroundColor: Colors.white,
            ),
            onPressed: () async {
              Navigator.of(ctx).pop();
              final defaultUrl = resolveDefaultServerUrl();
              _urlController.text = defaultUrl;
              try {
                final info = await ref.read(authRepositoryProvider).testConnection(defaultUrl);
                if (info.defaultUsername != null && info.defaultUsername!.isNotEmpty && mounted) {
                  _userController.text = info.defaultUsername!;
                }
              } catch (_) {}
            },
            child: const Text('Use Discovered Server'),
          ),
        ],
      ),
    );
  }

  Future<void> _handleConnect() async {
    if (!_formKey.currentState!.validate()) return;

    final url = _urlController.text.trim().isNotEmpty
        ? _urlController.text.trim()
        : resolveDefaultServerUrl();
    final user = _userController.text.trim();
    final pass = _passwordController.text;

    final success = await ref.read(authProvider.notifier).login(url, user, pass);
    if (success && mounted) {
      context.go('/books');
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authProvider);
    final horizontalPad = Responsive.horizontalPadding(context);

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 540),
            child: SingleChildScrollView(
              padding: EdgeInsets.symmetric(horizontal: horizontalPad, vertical: AppTokens.space24),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Brand Icon & Title
                    Center(
                      child: Container(
                        width: 56,
                        height: 56,
                        decoration: BoxDecoration(
                          color: AppTokens.charcoalInk,
                          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                        ),
                        child: const Icon(
                          Icons.bookmark_outline_rounded,
                          color: AppTokens.boneBackground,
                          size: 32,
                        ),
                      ),
                    ),
                    const SizedBox(height: AppTokens.space16),
                    Text(
                      'Connect to your Library',
                      textAlign: TextAlign.center,
                      style: AppTypography.titleSerif(fontSize: 28),
                    ),
                    const SizedBox(height: AppTokens.space8),
                    Text(
                      'Pair with your self-hosted Shelfd homelab daemon or Audiobookshelf companion server',
                      textAlign: TextAlign.center,
                      style: AppTypography.bodySans(fontSize: 14),
                    ),
                    const SizedBox(height: AppTokens.space32),

                    // Primary Action: QR Code Card
                    BentoCard(
                      onTap: _onScanQrPressed,
                      child: Column(
                        children: [
                          Container(
                            width: 56,
                            height: 56,
                            decoration: BoxDecoration(
                              color: AppTokens.boneContainer,
                              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                            ),
                            child: const Icon(
                              Icons.qr_code_scanner_rounded,
                              size: 32,
                              color: AppTokens.charcoalInk,
                            ),
                          ),
                          const SizedBox(height: AppTokens.space12),
                          Text(
                            'Scan Server QR Code',
                            style: AppTypography.titleSerif(fontSize: 18),
                          ),
                          const SizedBox(height: AppTokens.space4),
                          Text(
                            'Instant pairing via camera or mobile scanner',
                            style: AppTypography.bodySans(fontSize: 13),
                          ),
                        ],
                      ),
                    ),

                    const SizedBox(height: AppTokens.space24),

                    // Divider
                    Row(
                      children: [
                        const Expanded(child: Divider(color: AppTokens.crispBorder)),
                        Padding(
                          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
                          child: Text(
                            'Or connect manually',
                            style: AppTypography.labelCaps(fontSize: 11),
                          ),
                        ),
                        const Expanded(child: Divider(color: AppTokens.crispBorder)),
                      ],
                    ),

                    const SizedBox(height: AppTokens.space24),

                    // Error banner
                    if (authState.error != null) ...[
                      Container(
                        padding: const EdgeInsets.all(AppTokens.space12),
                        decoration: BoxDecoration(
                          color: const Color(0xFFFFEBE8),
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          border: Border.all(color: const Color(0xFFFFC9C2)),
                        ),
                        child: Text(
                          authState.error!,
                          style: const TextStyle(color: Color(0xFFC92A2A), fontSize: 13),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space16),
                    ],

                    // Manual Inputs
                    TextFormField(
                      controller: _urlController,
                      decoration: InputDecoration(
                        labelText: 'Server URL',
                        hintText: 'http://192.168.1.100:8080',
                        helperText: kIsWeb ? 'Connected to self-hosted API origin' : null,
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          borderSide: const BorderSide(color: AppTokens.crispBorder),
                        ),
                        prefixIcon: const Icon(Icons.lan_outlined),
                      ),
                      validator: (v) => (v == null || v.isEmpty) ? 'Server URL required' : null,
                    ),
                    const SizedBox(height: AppTokens.space16),

                    TextFormField(
                      controller: _userController,
                      decoration: InputDecoration(
                        labelText: 'Username',
                        hintText: 'admin',
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          borderSide: const BorderSide(color: AppTokens.crispBorder),
                        ),
                        prefixIcon: const Icon(Icons.person_outline),
                      ),
                      validator: (v) => (v == null || v.isEmpty) ? 'Username required' : null,
                    ),
                    const SizedBox(height: AppTokens.space16),

                    TextFormField(
                      controller: _passwordController,
                      obscureText: _obscurePassword,
                      decoration: InputDecoration(
                        labelText: 'Password',
                        hintText: '••••••••',
                        helperText: kIsWeb ? 'Check container logs on first startup for generated password' : null,
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          borderSide: const BorderSide(color: AppTokens.crispBorder),
                        ),
                        prefixIcon: const Icon(Icons.lock_outline),
                        suffixIcon: IconButton(
                          icon: Icon(_obscurePassword ? Icons.visibility_off : Icons.visibility),
                          onPressed: () => setState(() => _obscurePassword = !_obscurePassword),
                        ),
                      ),
                      validator: (v) => (v == null || v.isEmpty) ? 'Password required' : null,
                    ),
                    const SizedBox(height: AppTokens.space24),

                    // Connect CTA Button (Min 48px touch target)
                    PrimaryButton(
                      label: 'Connect & Sync Library',
                      isLoading: authState.isLoading,
                      onPressed: authState.isLoading ? null : _handleConnect,
                    ),

                    const SizedBox(height: AppTokens.space32),

                    // Privacy Assurance Footer
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        const Icon(Icons.shield_outlined, size: 16, color: AppTokens.mutedCopy),
                        const SizedBox(width: AppTokens.space8),
                        Flexible(
                          child: Text(
                            'Direct connection to your private server. No third-party cloud.',
                            style: AppTypography.bodySans(fontSize: 12),
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
    );
  }
}
