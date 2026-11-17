import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/yoto.dart';
import '../../data/repositories/yoto_repository.dart';
import 'providers.dart';

final yotoRepositoryProvider = Provider<YotoRepository>((ref) {
  return YotoRepository(
    apiService: ref.watch(apiServiceProvider),
    storageService: ref.watch(storageServiceProvider),
  );
});

class YotoAccountState {
  final YotoStatus status;
  final bool isLoading;
  final String? errorMessage;
  final String? successMessage;

  const YotoAccountState({
    this.status = const YotoStatus(),
    this.isLoading = false,
    this.errorMessage,
    this.successMessage,
  });

  bool get isLinked => status.isLinked;

  YotoAccountState copyWith({
    YotoStatus? status,
    bool? isLoading,
    String? errorMessage,
    String? successMessage,
  }) {
    return YotoAccountState(
      status: status ?? this.status,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
      successMessage: successMessage,
    );
  }
}

class YotoAccountNotifier extends Notifier<YotoAccountState> {
  bool _isDisposed = false;

  @override
  YotoAccountState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    Future.microtask(loadStatus);
    return const YotoAccountState(isLoading: true);
  }

  Future<void> loadStatus() async {
    if (_isDisposed) return;
    state = state.copyWith(isLoading: true, errorMessage: null);
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.getStatus();
      if (!_isDisposed) {
        state = state.copyWith(status: status, isLoading: false);
      }
    } catch (e) {
      if (!_isDisposed) {
        state = state.copyWith(
          isLoading: false,
          errorMessage: 'Failed to load Yoto account status: $e',
        );
      }
    }
  }

  Future<String> buildAuthUrl({
    required String clientId,
    required String redirectUri,
  }) async {
    final repo = ref.read(yotoRepositoryProvider);
    return repo.buildAuthorizationUrl(
      clientId: clientId,
      redirectUri: redirectUri,
      authBaseUrl: state.status.authBaseUrl,
    );
  }

  Future<bool> exchangeCode({
    required String code,
    String? clientId,
    String? redirectUri,
  }) async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.exchangeOAuthCode(
        code: code,
        clientId: clientId,
        redirectUri: redirectUri,
      );
      state = state.copyWith(
        status: status,
        isLoading: false,
        successMessage: 'Yoto account linked successfully!',
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to link Yoto account: $e',
      );
      return false;
    }
  }

  Future<bool> linkWithTokens({
    required String accessToken,
    String? refreshToken,
    String? clientId,
  }) async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.linkWithTokens(
        accessToken: accessToken,
        refreshToken: refreshToken,
        clientId: clientId,
      );
      state = state.copyWith(
        status: status,
        isLoading: false,
        successMessage: 'Yoto account linked successfully!',
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to link Yoto account: $e',
      );
      return false;
    }
  }

  Future<void> unlink() async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      await repo.unlinkAccount();
      state = state.copyWith(
        status: YotoStatus(
          isLinked: false,
          defaultClientId: state.status.defaultClientId,
          authBaseUrl: state.status.authBaseUrl,
        ),
        isLoading: false,
        successMessage: 'Yoto account unlinked.',
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to unlink Yoto account: $e',
      );
    }
  }
}

final yotoAccountProvider =
    NotifierProvider<YotoAccountNotifier, YotoAccountState>(
  YotoAccountNotifier.new,
);

final yotoPlaylistsProvider = FutureProvider.autoDispose<List<YotoPlaylist>>((ref) async {
  final repo = ref.watch(yotoRepositoryProvider);
  return repo.getPlaylists();
});

final yotoIconsProvider = FutureProvider.autoDispose<YotoIconsCatalog>((ref) async {
  final repo = ref.watch(yotoRepositoryProvider);
  return repo.getIcons();
});
