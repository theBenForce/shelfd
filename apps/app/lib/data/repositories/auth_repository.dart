import '../models/connect_info.dart';
import '../models/user.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

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
      await apiService.getMe();
      return true;
    } catch (_) {
      await storageService.clearServerConnection();
      return false;
    }
  }

  Future<ServerConnectInfo> testConnection(String serverUrl) async {
    apiService.updateConnection(newBaseUrl: serverUrl);
    return await apiService.getConnectInfo();
  }

  Future<User> login(String serverUrl, String username, String password) async {
    apiService.updateConnection(newBaseUrl: serverUrl);
    final token = await apiService.login(username, password);
    await storageService.saveServerConnection(serverUrl, token);
    return await apiService.getMe();
  }

  Future<void> logout() async {
    await storageService.clearServerConnection();
    apiService.updateConnection(newToken: '');
  }
}
