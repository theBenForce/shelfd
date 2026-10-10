import 'dart:io';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/ui/features/upload/directory_scanner.dart';
import 'package:shelf/ui/state/upload_provider.dart';

void main() {
  group('Directory Scanner & Sibling Cover Association Tests', () {
    test('isSupportedBook recognizes ebooks and audiobooks', () {
      expect(isSupportedBook('novel.epub'), isTrue);
      expect(isSupportedBook('novel.EPUB'), isTrue);
      expect(isSupportedBook('audio.m4b'), isTrue);
      expect(isSupportedBook('audio.mp3'), isTrue);
      expect(isSupportedBook('audio.m4a'), isTrue);
      expect(isSupportedBook('audio.flac'), isTrue);
      expect(isSupportedBook('audio.MP3'), isTrue);

      expect(isSupportedBook('document.pdf'), isFalse);
      expect(isSupportedBook('notes.txt'), isFalse);
      expect(isSupportedBook('cover.jpg'), isFalse);
    });

    test('isSupportedCover recognizes image extensions', () {
      expect(isSupportedCover('cover.jpg'), isTrue);
      expect(isSupportedCover('cover.jpeg'), isTrue);
      expect(isSupportedCover('folder.png'), isTrue);
      expect(isSupportedCover('art.webp'), isTrue);
      expect(isSupportedCover('art.WEBP'), isTrue);

      expect(isSupportedCover('book.epub'), isFalse);
      expect(isSupportedCover('audio.m4b'), isFalse);
      expect(isSupportedCover('file.txt'), isFalse);
    });

    test('matchLooseFilesWithCovers pairs matching base names', () async {
      final books = [
        PickedEpubFile(
          name: 'Project Hail Mary.m4b',
          path: '/path/Project Hail Mary.m4b',
          readBytes: () async => [1, 2, 3],
        ),
        PickedEpubFile(
          name: 'The Martian.epub',
          path: '/path/The Martian.epub',
          readBytes: () async => [4, 5, 6],
        ),
      ];

      final covers = [
        PickedEpubFile(
          name: 'Project Hail Mary.jpg',
          path: '/path/Project Hail Mary.jpg',
          readBytes: () async => [10, 20],
        ),
        PickedEpubFile(
          name: 'The Martian.png',
          path: '/path/The Martian.png',
          readBytes: () async => [30, 40],
        ),
      ];

      final matched = matchLooseFilesWithCovers(books, covers);
      expect(matched.length, 2);

      final hailMary = matched.firstWhere((b) => b.name == 'Project Hail Mary.m4b');
      expect(hailMary.coverName, 'Project Hail Mary.jpg');
      final hailMaryCoverBytes = await hailMary.getCoverBytes();
      expect(hailMaryCoverBytes, [10, 20]);

      final martian = matched.firstWhere((b) => b.name == 'The Martian.epub');
      expect(martian.coverName, 'The Martian.png');
      final martianCoverBytes = await martian.getCoverBytes();
      expect(martianCoverBytes, [30, 40]);
    });

    test('matchLooseFilesWithCovers pairs standard cover.jpg or lone cover', () async {
      final books = [
        PickedEpubFile(
          name: 'Dune.m4b',
          path: '/downloads/Dune.m4b',
          readBytes: () async => [1, 1, 1],
        ),
      ];

      final covers = [
        PickedEpubFile(
          name: 'cover.jpg',
          path: '/downloads/cover.jpg',
          readBytes: () async => [99, 98],
        ),
      ];

      final matched = matchLooseFilesWithCovers(books, covers);
      expect(matched.length, 1);
      expect(matched.first.coverName, 'cover.jpg');
      final coverBytes = await matched.first.getCoverBytes();
      expect(coverBytes, [99, 98]);
    });

    test('scanPathForEpubs discovers nested audiobooks and sibling covers in directory', () async {
      final tempDir = await Directory.systemTemp.createTemp('shelfd_scan_test_');

      try {
        // Create folder hierarchy:
        // tempDir/
        //   Frank Herbert/
        //     Dune/
        //       Dune.m4b
        //       cover.jpg
        //   Andy Weir/
        //     Project Hail Mary/
        //       Project Hail Mary.epub
        //       Project Hail Mary.png
        final duneDir = Directory('${tempDir.path}/Frank Herbert/Dune');
        await duneDir.create(recursive: true);
        final duneFile = File('${duneDir.path}/Dune.m4b');
        await duneFile.writeAsBytes([1, 2, 3, 4]);
        final duneCover = File('${duneDir.path}/cover.jpg');
        await duneCover.writeAsBytes([255, 216, 255]); // JPEG magic header

        final weirDir = Directory('${tempDir.path}/Andy Weir/Project Hail Mary');
        await weirDir.create(recursive: true);
        final weirFile = File('${weirDir.path}/Project Hail Mary.epub');
        await weirFile.writeAsBytes([80, 75, 3, 4]); // ZIP magic header
        final weirCover = File('${weirDir.path}/Project Hail Mary.png');
        await weirCover.writeAsBytes([137, 80, 78, 71]); // PNG magic header

        final scanned = await scanPathForEpubs(tempDir.path);
        expect(scanned.length, 2);

        final dunePicked = scanned.firstWhere((f) => f.name == 'Dune.m4b');
        expect(dunePicked.coverName, 'cover.jpg');
        expect(await dunePicked.getCoverBytes(), [255, 216, 255]);

        final weirPicked = scanned.firstWhere((f) => f.name == 'Project Hail Mary.epub');
        expect(weirPicked.coverName, 'Project Hail Mary.png');
        expect(await weirPicked.getCoverBytes(), [137, 80, 78, 71]);
      } finally {
        await tempDir.delete(recursive: true);
      }
    });
  });
}
