import '../../ui/features/reader/html_view.dart';
import '../models/connect_info.dart';
import '../models/user.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';
import '../utils/url_normalizer.dart';

class AuthRepository {
  final ApiService apiService;
  final StorageService storageService;

  AuthRepository({
    required this.apiService,
    required this.storageService,
  });

  Future<bool> checkInitialAuth() async {
    final serverUrl = storageService.getServerUrl();
    final token = storageService.getAuthToken();

    if (serverUrl == null || token == null || serverUrl.isEmpty || token.isEmpty) {
      return false;
    }

    apiService.updateConnection(newBaseUrl: serverUrl, newToken: token);
    try {
      final user = await apiService.getMe();
      if (user.username.isNotEmpty) {
        await storageService.saveSavedUsername(user.username);
      }
      return true;
    } on ApiException catch (e) {
      if (e.statusCode == 401) {
        await storageService.clearAuthToken();
        apiService.updateConnection(newToken: '');
        return false;
      }
      return true;
    } catch (_) {
      return true;
    }
  }

  Future<ServerConnectInfo> testConnection(String serverUrl) async {
    final normalized = UrlNormalizer.normalizeServerUrl(serverUrl);
    final probeApi = ApiService(
      baseUrl: normalized.url,
      client: apiService.client,
    );
    return await probeApi.getConnectInfo();
  }

  Future<User> login(
    String serverUrl,
    String username,
    String password, {
    String? deviceName,
  }) async {
    final normalized = UrlNormalizer.normalizeServerUrl(serverUrl);
    final probeApi = ApiService(
      baseUrl: normalized.url,
      client: apiService.client,
    );
    final connectInfo = await probeApi.getConnectInfo();
    if (connectInfo.serverName.trim().isEmpty) {
      throw const ApiException(
        404,
        'Endpoint did not return valid Shelfd server metadata.',
      );
    }
    final result = await probeApi.loginWithDevice(
      username,
      password,
      deviceName: deviceName,
    );
    apiService.updateConnection(
      newBaseUrl: normalized.url,
      newToken: result.token,
    );
    final resolvedUser = result.user ?? await apiService.getMe();
    await storageService.saveServerConnection(
      normalized.url,
      result.token,
      username: resolvedUser.username,
      tokenId: result.tokenId,
    );
    return resolvedUser;
  }

  Future<User> pairWithUri(
    String rawPairInput, {
    required String deviceName,
    bool rejectLoopback = true,
  }) async {
    final parsed = UrlNormalizer.parsePairingInput(
      rawPairInput,
      rejectLoopback: rejectLoopback,
    );
    final probeApi = ApiService(
      baseUrl: parsed.serverUrl,
      client: apiService.client,
    );
    final result = await probeApi.exchangePairing(
      parsed.code,
      deviceName: deviceName,
    );
    apiService.updateConnection(
      newBaseUrl: parsed.serverUrl,
      newToken: result.token,
    );
    await storageService.saveServerConnection(
      parsed.serverUrl,
      result.token,
      username: result.user.username,
      tokenId: result.tokenId,
    );
    return result.user;
  }

  Future<void> changePassword(String currentPassword, String newPassword) async {
    await apiService.changePassword(currentPassword, newPassword);
  }

  Future<void> logout({bool clearServerUrl = false}) async {
    final tokenId = storageService.getAuthTokenId();
    final activeToken = apiService.token ?? storageService.getAuthToken();
    if (tokenId != null &&
        tokenId.isNotEmpty &&
        activeToken != null &&
        activeToken.isNotEmpty) {
      apiService.updateConnection(newToken: activeToken);
      try {
        await apiService.deleteApiToken(tokenId, notifyUnauthorized: false);
      } catch (_) {}
    }
    try {
      await clearHtmlAuthCookies();
    } catch (_) {}
    if (clearServerUrl) {
      await storageService.clearServerConnection();
    } else {
      await storageService.clearAuthToken();
    }
    apiService.updateConnection(newToken: '');
  }
}
