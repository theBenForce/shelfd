import '../../state/upload_provider.dart';

const supportedBookExtensions = {'.epub', '.m4b', '.mp3', '.m4a', '.flac'};
const supportedCoverExtensions = {'.jpg', '.jpeg', '.png', '.webp'};

bool isSupportedBook(String path) {
  final lower = path.toLowerCase();
  return supportedBookExtensions.any((ext) => lower.endsWith(ext));
}

bool isSupportedCover(String path) {
  final lower = path.toLowerCase();
  return supportedCoverExtensions.any((ext) => lower.endsWith(ext));
}

bool isDirectoryPath(String path) {
  return false;
}

Future<List<PickedEpubFile>> scanPathForEpubs(String path) async {
  return [];
}

Future<List<PickedEpubFile>> pickFolderForEpubs() async {
  return [];
}

List<PickedEpubFile> matchLooseFilesWithCovers(
  List<PickedEpubFile> books,
  List<PickedEpubFile> covers,
) {
  return books;
}
