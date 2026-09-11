import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../data/models/author.dart';
import '../../data/models/book.dart';
import '../../data/models/genre.dart';
import '../../data/models/series.dart';
import '../../data/models/topic.dart';
import '../state/providers.dart';
import 'responsive.dart';
import 'tokens.dart';
import 'typography.dart';

/// Resolves a book ID to its human-readable title if found in the loaded catalog.
String? resolveBookTitle(String? bookOrId, List<Book> books) {
  if (bookOrId == null || bookOrId.isEmpty) return bookOrId;
  final match = books.where((b) => b.id == bookOrId).firstOrNull;
  return match != null && match.title.isNotEmpty ? match.title : bookOrId;
}

class BentoCard extends StatelessWidget {
  final Widget child;
  final VoidCallback? onTap;
  final EdgeInsetsGeometry padding;
  final Color? backgroundColor;
  final double? width;
  final double? height;

  const BentoCard({
    super.key,
    required this.child,
    this.onTap,
    this.padding = const EdgeInsets.all(AppTokens.space16),
    this.backgroundColor,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cardWidget = Card(
      elevation: 0,
      margin: EdgeInsets.zero,
      color: backgroundColor ?? theme.cardColor,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        side: BorderSide(color: theme.dividerColor),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          width: width,
          height: height,
          padding: padding,
          child: child,
        ),
      ),
    );

    return cardWidget;
  }
}

class FilterPillItem {
  final String id;
  final String label;

  const FilterPillItem({required this.id, required this.label});
}

class FilterPillsRow extends StatelessWidget {
  final List<FilterPillItem> items;
  final String selectedId;
  final ValueChanged<String> onSelected;

  const FilterPillsRow({
    super.key,
    required this.items,
    required this.selectedId,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: items.map((item) {
          final isSelected = item.id == selectedId;
          return Padding(
            padding: const EdgeInsets.only(right: AppTokens.space8),
            child: ChoiceChip(
              label: Text(
                item.label,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                  color: isSelected ? Colors.white : AppTokens.mutedCopy,
                ),
              ),
              selected: isSelected,
              onSelected: (_) => onSelected(item.id),
              selectedColor: AppTokens.charcoalInk,
              backgroundColor: AppTokens.boneSurface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                side: BorderSide(
                  color: isSelected ? AppTokens.charcoalInk : AppTokens.crispBorder,
                ),
              ),
              showCheckmark: false,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            ),
          );
        }).toList(),
      ),
    );
  }
}

class StatusBadge extends StatelessWidget {
  final String label;
  final Color backgroundColor;
  final Color textColor;

  const StatusBadge({
    super.key,
    required this.label,
    this.backgroundColor = AppTokens.matchBadgeBg,
    this.textColor = AppTokens.matchBadgeText,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppTokens.space8, vertical: AppTokens.space4),
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: textColor,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}

class AuthorAvatar extends StatelessWidget {
  final String name;
  final String? photoUrl;
  final double size;

  const AuthorAvatar({
    super.key,
    required this.name,
    this.photoUrl,
    this.size = 56,
  });

  String get initials {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.isEmpty || parts[0].isEmpty) return '?';
    if (parts.length == 1) return parts[0][0].toUpperCase();
    return (parts[0][0] + parts.last[0]).toUpperCase();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: AppTokens.boneContainer,
        shape: BoxShape.circle,
        border: Border.all(color: AppTokens.crispBorder),
      ),
      clipBehavior: Clip.antiAlias,
      child: photoUrl != null && photoUrl!.isNotEmpty
          ? Image.network(
              photoUrl!,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) => _buildInitials(),
            )
          : _buildInitials(),
    );
  }

  Widget _buildInitials() {
    return Center(
      child: Text(
        initials,
        style: AppTypography.titleSerif(
          fontSize: size * 0.38,
          fontWeight: FontWeight.w700,
          color: AppTokens.charcoalInk,
        ),
      ),
    );
  }
}

class PrimaryButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool isLoading;
  final IconData? icon;
  final Color? backgroundColor;
  final Color? foregroundColor;

  const PrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.isLoading = false,
    this.icon,
    this.backgroundColor,
    this.foregroundColor,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: AppTokens.minTouchTarget,
      child: ElevatedButton(
        style: ElevatedButton.styleFrom(
          backgroundColor: backgroundColor ?? AppTokens.charcoalInk,
          foregroundColor: foregroundColor ?? Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space20),
        ),
        onPressed: isLoading ? null : onPressed,
        child: isLoading
            ? const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (icon != null) ...[
                    Icon(icon, size: 18),
                    const SizedBox(width: AppTokens.space8),
                  ],
                  Text(
                    label,
                    style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                  ),
                ],
              ),
      ),
    );
  }
}

class ShelfdTopBar extends StatelessWidget implements PreferredSizeWidget {
  final String title;
  final String? subtitle;
  final List<Widget>? actions;
  final Widget? leading;

  const ShelfdTopBar({
    super.key,
    required this.title,
    this.subtitle,
    this.actions,
    this.leading,
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight + 8);

  @override
  Widget build(BuildContext context) {
    return AppBar(
      leading: leading,
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(title, style: AppTypography.titleSerif(fontSize: 20)),
          if (subtitle != null) ...[
            const SizedBox(height: 2),
            Row(
              children: [
                Container(
                  width: 6,
                  height: 6,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    subtitle!,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodySans(fontSize: 11, color: AppTokens.mutedCopy),
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
      actions: actions,
      bottom: PreferredSize(
        preferredSize: const Size.fromHeight(1),
        child: Container(color: AppTokens.crispBorder, height: 1),
      ),
    );
  }
}

class ShelfdUploadsBadgeButton extends ConsumerWidget {
  const ShelfdUploadsBadgeButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final queue = ref.watch(queueProvider);
    final count = queue.status?.stagedUploads ?? 0;
    return IconButton(
      tooltip: count > 0 ? '$count staged uploads' : 'Uploads',
      icon: Badge(
        isLabelVisible: count > 0,
        label: Text('$count', style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 10)),
        backgroundColor: const Color(0xFFF08C00),
        textColor: Colors.white,
        child: const Icon(Icons.cloud_upload_outlined, size: 22),
      ),
      onPressed: () => context.go('/uploads'),
    );
  }
}

class ShelfdBottomNav extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;

  const ShelfdBottomNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
    this.currentPath,
    this.onNavigate,
  });

  int _calculateIndex() {
    if (currentPath != null) {
      final p = currentPath!;
      if (p.startsWith('/series')) return 1;
      if (p.startsWith('/authors') || p.startsWith('/author')) return 2;
      if (p.startsWith('/search')) return 3;
      if (p.startsWith('/settings')) return 4;
      return 0; // /books, /library, default
    }
    return currentIndex.clamp(0, 4);
  }

  void _handleTap(int index) {
    if (onNavigate != null) {
      switch (index) {
        case 0:
          onNavigate!('/books');
          return;
        case 1:
          onNavigate!('/series');
          return;
        case 2:
          onNavigate!('/authors');
          return;
        case 3:
          onNavigate!('/search');
          return;
        case 4:
          onNavigate!('/settings');
          return;
      }
    }
    onTap(index);
  }

  @override
  Widget build(BuildContext context) {
    final idx = _calculateIndex();
    return Container(
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: AppTokens.crispBorder, width: 1)),
      ),
      child: NavigationBar(
        selectedIndex: idx,
        onDestinationSelected: _handleTap,
        backgroundColor: Theme.of(context).scaffoldBackgroundColor,
        elevation: 0,
        height: 64,
        indicatorColor: AppTokens.boneContainer,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.menu_book_outlined),
            selectedIcon: Icon(Icons.menu_book_rounded, color: AppTokens.charcoalInk),
            label: 'Books',
          ),
          NavigationDestination(
            icon: Icon(Icons.collections_bookmark_outlined),
            selectedIcon: Icon(Icons.collections_bookmark_rounded, color: AppTokens.charcoalInk),
            label: 'Series',
          ),
          NavigationDestination(
            icon: Icon(Icons.people_outline_rounded),
            selectedIcon: Icon(Icons.people_rounded, color: AppTokens.charcoalInk),
            label: 'Authors',
          ),
          NavigationDestination(
            icon: Icon(Icons.saved_search_outlined),
            selectedIcon: Icon(Icons.saved_search_rounded, color: AppTokens.charcoalInk),
            label: 'Search',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            selectedIcon: Icon(Icons.settings_rounded, color: AppTokens.charcoalInk),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}

class ShelfdSideNav extends ConsumerStatefulWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;
  final VoidCallback? onRescan;
  final VoidCallback? onUpload;
  final bool isRescanning;

  const ShelfdSideNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
    this.currentPath,
    this.onNavigate,
    this.onRescan,
    this.onUpload,
    this.isRescanning = false,
  });

  @override
  ConsumerState<ShelfdSideNav> createState() => _ShelfdSideNavState();
}

class _ShelfdSideNavState extends ConsumerState<ShelfdSideNav> {
  bool _isLibraryExpanded = true;

  void _navigate(String path, int fallbackIndex) {
    if (widget.onNavigate != null) {
      widget.onNavigate!(path);
    } else {
      widget.onTap(fallbackIndex);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final p = widget.currentPath ?? '';
    final isBooks = p.startsWith('/books') || p == '/library' || p.startsWith('/book/');
    final isSeries = p.startsWith('/series');
    final isAuthors = p.startsWith('/authors') || p.startsWith('/author');
    final isGenres = p.startsWith('/genres') || p.startsWith('/genre');
    final isTopics = p.startsWith('/topics') || p.startsWith('/topic');
    final isSearch = p.startsWith('/search') || (widget.currentPath == null && widget.currentIndex == 1);
    final isUploads = p.startsWith('/uploads');
    final isSettings = p.startsWith('/settings') || (widget.currentPath == null && widget.currentIndex == 2);
    final isLibraryActive = isBooks || isSeries || isAuthors || isGenres || isTopics || (widget.currentPath == null && widget.currentIndex == 0);
    final queueStatus = ref.watch(queueProvider).status;
    final stagedCount = queueStatus?.stagedUploads ?? 0;

    return Container(
      width: AppTokens.sidebarWidth,
      height: double.infinity,
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: const Border(
          right: BorderSide(color: AppTokens.crispBorder, width: 1),
        ),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space20,
        vertical: AppTokens.space24,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Brand Logo & Title
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: AppTokens.charcoalInk,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
                child: const Icon(
                  Icons.auto_stories_rounded,
                  color: AppTokens.boneSurface,
                  size: 20,
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'Shelfd',
                      style: AppTypography.titleSerif(
                        fontSize: 22,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    Text(
                      'Your Digital Vellum',
                      style: AppTypography.bodySans(
                        fontSize: 11,
                        color: AppTokens.mutedCopy,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space24),

          // Scrollable Navigation Links
          Expanded(
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Collapsible Library Header Item
                  _SideNavItem(
                    icon: Icons.local_library_outlined,
                    selectedIcon: Icons.local_library_rounded,
                    label: 'Library',
                    isSelected: isLibraryActive && !_isLibraryExpanded,
                    trailing: IconButton(
                      visualDensity: VisualDensity.compact,
                      icon: Icon(
                        _isLibraryExpanded ? Icons.keyboard_arrow_down_rounded : Icons.keyboard_arrow_right_rounded,
                        size: 20,
                        color: AppTokens.mutedCopy,
                      ),
                      onPressed: () {
                        setState(() {
                          _isLibraryExpanded = !_isLibraryExpanded;
                        });
                      },
                    ),
                    onTap: () {
                      setState(() {
                        _isLibraryExpanded = !_isLibraryExpanded;
                      });
                      if (_isLibraryExpanded) {
                        _navigate('/books', 0);
                      }
                    },
                  ),

                  // Indented Sub-items under Library
                  if (_isLibraryExpanded) ...[
                    _SideNavSubItem(
                      icon: Icons.menu_book_outlined,
                      selectedIcon: Icons.menu_book_rounded,
                      label: 'Books',
                      isSelected: isBooks || (isLibraryActive && !isSeries && !isAuthors && !isGenres && !isTopics),
                      onTap: () => _navigate('/books', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.collections_bookmark_outlined,
                      selectedIcon: Icons.collections_bookmark_rounded,
                      label: 'Series',
                      isSelected: isSeries,
                      onTap: () => _navigate('/series', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.people_outline_rounded,
                      selectedIcon: Icons.people_rounded,
                      label: 'Authors',
                      isSelected: isAuthors,
                      onTap: () => _navigate('/authors', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.category_outlined,
                      selectedIcon: Icons.category_rounded,
                      label: 'Genres',
                      isSelected: isGenres,
                      onTap: () => _navigate('/genres', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.label_outline_rounded,
                      selectedIcon: Icons.label_rounded,
                      label: 'Topics',
                      isSelected: isTopics,
                      onTap: () => _navigate('/topics', 0),
                    ),
                  ],
                  const SizedBox(height: AppTokens.space8),

                  // Top-level Navigation Links
                  _SideNavItem(
                    icon: Icons.saved_search_outlined,
                    selectedIcon: Icons.saved_search_rounded,
                    label: 'Semantic Search',
                    isSelected: isSearch,
                    onTap: () => _navigate('/search', 1),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  _SideNavItem(
                    icon: Icons.cloud_upload_outlined,
                    selectedIcon: Icons.cloud_upload_rounded,
                    label: 'Uploads',
                    isSelected: isUploads,
                    trailing: stagedCount > 0
                        ? Container(
                            padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                            decoration: BoxDecoration(
                              color: const Color(0xFFFFF9DB),
                              border: Border.all(color: const Color(0xFFFFE066)),
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Text(
                              '$stagedCount',
                              style: const TextStyle(
                                fontSize: 11,
                                fontWeight: FontWeight.w700,
                                color: Color(0xFFF08C00),
                              ),
                            ),
                          )
                        : null,
                    onTap: () => _navigate('/uploads', 3),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  _SideNavItem(
                    icon: Icons.settings_outlined,
                    selectedIcon: Icons.settings_rounded,
                    label: 'Settings',
                    isSelected: isSettings,
                    onTap: () => _navigate('/settings', 2),
                  ),
                ],
              ),
            ),
          ),

          const SizedBox(height: AppTokens.space12),

          // Live AI indexing queue progress card
          const _QueueStatusCard(),
          const SizedBox(height: AppTokens.space12),

          // Rescan Library Button (if callback provided)
          if (widget.onRescan != null) ...[
            SizedBox(
              width: double.infinity,
              height: 40,
              child: OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  backgroundColor: AppTokens.charcoalInk,
                  foregroundColor: Colors.white,
                  side: BorderSide.none,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
                ),
                onPressed: widget.isRescanning ? null : widget.onRescan,
                icon: widget.isRescanning
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                      )
                    : const Icon(Icons.sync_rounded, size: 18),
                label: Text(
                  widget.isRescanning ? 'Scanning...' : 'Rescan Library',
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space16),
          ],

          // Homelab NAS connection status badge
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: Row(
              children: [
                Container(
                  width: 8,
                  height: 8,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'Homelab NAS - Connected',
                    style: AppTypography.bodySans(
                      fontSize: 11,
                      fontWeight: FontWeight.w500,
                      color: AppTokens.mutedCopy,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SideNavItem extends StatelessWidget {
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;
  final Widget? trailing;

  const _SideNavItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.isSelected,
    required this.onTap,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: isSelected ? AppTokens.boneContainer : Colors.transparent,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          height: AppTokens.minTouchTarget,
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: isSelected ? Border.all(color: AppTokens.crispBorder) : null,
          ),
          child: Row(
            children: [
              Icon(
                isSelected ? selectedIcon : icon,
                color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                size: 22,
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Text(
                  label,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              ?trailing,
              if (isSelected && trailing == null)
                Container(
                  width: 4,
                  height: 16,
                  decoration: BoxDecoration(
                    color: AppTokens.charcoalInk,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SideNavSubItem extends StatelessWidget {
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _SideNavSubItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: AppTokens.space16, top: 2, bottom: 2),
      child: Material(
        color: isSelected ? AppTokens.boneContainer : Colors.transparent,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          child: Container(
            height: 36,
            padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: isSelected ? Border.all(color: AppTokens.crispBorder) : null,
            ),
            child: Row(
              children: [
                Icon(
                  isSelected ? selectedIcon : icon,
                  color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                  size: 18,
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    label,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                      color: isSelected ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                if (isSelected)
                  Container(
                    width: 4,
                    height: 14,
                    decoration: BoxDecoration(
                      color: AppTokens.charcoalInk,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _QueueStatusCard extends ConsumerWidget {
  const _QueueStatusCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final queueState = ref.watch(queueProvider);
    final status = queueState.status;

    if (status == null) {
      if (queueState.isLoading) {
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: AppTokens.boneContainer.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: AppTokens.crispBorder.withValues(alpha: 0.6)),
          ),
          child: Row(
            children: [
              const SizedBox(
                width: 12,
                height: 12,
                child: CircularProgressIndicator(strokeWidth: 1.5, color: AppTokens.mutedCopy),
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Checking queue...',
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    color: AppTokens.mutedCopy,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        );
      }
      return const SizedBox.shrink();
    }

    final isActive = status.isActive || status.pendingChapters > 0 || status.pendingUploads > 0;

    if (!isActive) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: AppTokens.boneContainer.withValues(alpha: 0.5),
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          border: Border.all(color: AppTokens.crispBorder.withValues(alpha: 0.6)),
        ),
        child: Row(
          children: [
            const Icon(Icons.check_circle_outline_rounded, size: 14, color: Color(0xFF2B8A3E)),
            const SizedBox(width: AppTokens.space8),
            Expanded(
              child: Text(
                'AI Catalog Synced (${status.totalChapters} passages)',
                style: AppTypography.bodySans(
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                  color: AppTokens.mutedCopy,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      );
    }

    final libraryState = ref.watch(libraryProvider);
    final resolvedBook = resolveBookTitle(status.currentBook, libraryState.books);
    final progress = (status.progressPercent / 100.0).clamp(0.0, 1.0);
    final title = (resolvedBook != null && resolvedBook.isNotEmpty)
        ? resolvedBook
        : (status.pendingUploads > 0 ? 'Processing uploads...' : 'Indexing library passages...');

    final subtext = (status.currentChapter != null && status.currentChapter!.isNotEmpty)
        ? status.currentChapter!
        : '${status.indexedChapters} of ${status.totalChapters} indexed (${status.progressPercent.toStringAsFixed(0)}%)';

    return InkWell(
      onTap: () => ref.read(queueProvider.notifier).refresh(),
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Container(
        padding: const EdgeInsets.all(AppTokens.space12),
        decoration: BoxDecoration(
          color: AppTokens.boneSurface,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(color: AppTokens.crispBorder),
          boxShadow: const [
            BoxShadow(
              color: Color(0x0A000000),
              blurRadius: 4,
              offset: Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: const Color(0xFFFFF4E6),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: const Icon(
                    Icons.auto_awesome_rounded,
                    size: 14,
                    color: Color(0xFFD9480F),
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'AI Indexing',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: AppTokens.charcoalInk,
                    ),
                  ),
                ),
                Text(
                  '${status.progressPercent.toStringAsFixed(0)}%',
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: const Color(0xFFD9480F),
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
                backgroundColor: AppTokens.boneContainer,
                valueColor: const AlwaysStoppedAnimation<Color>(Color(0xFFD9480F)),
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              title,
              style: AppTypography.bodySans(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: AppTokens.charcoalInk,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 2),
            Text(
              subtext,
              style: AppTypography.bodySans(
                fontSize: 10,
                color: AppTokens.mutedCopy,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

class ShelfdAdaptiveScaffold extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onNavTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;
  final PreferredSizeWidget? appBar;
  final Widget body;
  final VoidCallback? onRescan;
  final VoidCallback? onUpload;
  final bool isRescanning;

  const ShelfdAdaptiveScaffold({
    super.key,
    required this.currentIndex,
    required this.onNavTap,
    this.currentPath,
    this.onNavigate,
    this.appBar,
    required this.body,
    this.onRescan,
    this.onUpload,
    this.isRescanning = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDesktop = Responsive.isDesktop(context);

    if (isDesktop) {
      return Scaffold(
        body: Row(
          children: [
            ShelfdSideNav(
              currentIndex: currentIndex,
              onTap: onNavTap,
              currentPath: currentPath,
              onNavigate: onNavigate,
              onRescan: onRescan,
              onUpload: onUpload,
              isRescanning: isRescanning,
            ),
            Expanded(
              child: body,
            ),
          ],
        ),
      );
    }

    return Scaffold(
      appBar: appBar,
      body: body,
      bottomNavigationBar: ShelfdBottomNav(
        currentIndex: currentIndex,
        onTap: onNavTap,
        currentPath: currentPath,
        onNavigate: onNavigate,
      ),
    );
  }
}

/// Unified, polymorphic grid card component for [LibraryGridItem].
class ShelfdGridCard extends StatelessWidget {
  final LibraryGridItem item;
  final VoidCallback onTap;

  const ShelfdGridCard({
    super.key,
    required this.item,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    switch (item) {
      case BookGridItem(:final book):
        return _buildBookCard(book);
      case SeriesGridItem(:final series):
        return _buildSeriesCard(series);
      case AuthorGridItem(:final author):
        return _buildAuthorCard(author);
      case GenreGridItem(:final genre):
        return _buildGenreCard(genre);
      case TopicGridItem(:final topic):
        return _buildTopicCard(topic);
    }
  }

  Widget _buildBookCard(Book book) {
    final progressPercent = (book.readingProgress * 100).round();

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(AppTokens.radiusMd),
                ),
                border: const Border(
                  bottom: BorderSide(color: AppTokens.crispBorder),
                ),
              ),
              child: ClipRRect(
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(AppTokens.radiusMd),
                ),
                child: book.coverUrl != null && book.coverUrl!.isNotEmpty
                    ? Image.network(
                        book.coverUrl!,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) =>
                            _CoverFallback(title: book.title),
                      )
                    : _CoverFallback(title: book.title),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  book.title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  book.authorDisplay,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(fontSize: 12),
                ),
                if (progressPercent > 0) ...[
                  const SizedBox(height: AppTokens.space8),
                  StatusBadge(
                    label: '$progressPercent% read',
                    backgroundColor: AppTokens.matchBadgeBg,
                    textColor: AppTokens.matchBadgeText,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSeriesCard(Series series) {
    final bookCountLabel = series.bookCount == 1 ? '1 Book' : '${series.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: Container(
              decoration: BoxDecoration(
                color: AppTokens.boneContainer,
                borderRadius: const BorderRadius.vertical(
                  top: Radius.circular(AppTokens.radiusMd),
                ),
                border: const Border(
                  bottom: BorderSide(color: AppTokens.crispBorder),
                ),
              ),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  ClipRRect(
                    borderRadius: const BorderRadius.vertical(
                      top: Radius.circular(AppTokens.radiusMd),
                    ),
                    child: series.coverUrl != null && series.coverUrl!.isNotEmpty
                        ? Image.network(
                            series.coverUrl!,
                            fit: BoxFit.cover,
                            errorBuilder: (context, error, stackTrace) =>
                                _SeriesFallback(name: series.name),
                          )
                        : _SeriesFallback(name: series.name),
                  ),
                  Positioned(
                    top: AppTokens.space8,
                    right: AppTokens.space8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                      decoration: BoxDecoration(
                        color: AppTokens.charcoalInk.withValues(alpha: 0.85),
                        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.collections_bookmark_outlined, size: 12, color: Colors.white),
                          const SizedBox(width: 4),
                          Text(
                            bookCountLabel,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  series.name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  'Series',
                  style: AppTypography.captionSans(
                    fontSize: 11,
                    color: AppTokens.mutedCopy,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAuthorCard(Author author) {
    final bookCountLabel = author.bookCount == 1 ? '1 Book' : '${author.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          AuthorAvatar(
            name: author.name,
            photoUrl: author.photoUrl,
            size: 72,
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            author.name,
            maxLines: 2,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space8),
          StatusBadge(
            label: bookCountLabel,
            backgroundColor: AppTokens.boneContainer,
            textColor: AppTokens.mutedCopy,
          ),
        ],
      ),
    );
  }

  Widget _buildGenreCard(Genre genre) {
    final bookCountLabel = genre.bookCount == 1 ? '1 Book' : '${genre.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              shape: BoxShape.circle,
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: const Icon(
              Icons.category_outlined,
              size: 28,
              color: AppTokens.charcoalInk,
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            genre.name,
            maxLines: 2,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space8),
          StatusBadge(
            label: bookCountLabel,
            backgroundColor: AppTokens.boneContainer,
            textColor: AppTokens.mutedCopy,
          ),
        ],
      ),
    );
  }

  Widget _buildTopicCard(Topic topic) {
    final bookCountLabel = topic.bookCount == 1 ? '1 Book' : '${topic.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              color: AppTokens.boneContainer,
              shape: BoxShape.circle,
              border: Border.all(color: AppTokens.crispBorder),
            ),
            child: const Icon(
              Icons.label_outline_rounded,
              size: 28,
              color: AppTokens.charcoalInk,
            ),
          ),
          const SizedBox(height: AppTokens.space12),
          Text(
            topic.name,
            maxLines: 2,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 14, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: AppTokens.space8),
          StatusBadge(
            label: bookCountLabel,
            backgroundColor: AppTokens.boneContainer,
            textColor: AppTokens.mutedCopy,
          ),
        ],
      ),
    );
  }
}

class _CoverFallback extends StatelessWidget {
  final String title;

  const _CoverFallback({required this.title});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.book_rounded, size: 36, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            title,
            maxLines: 3,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 12),
          ),
        ],
      ),
    );
  }
}

class _SeriesFallback extends StatelessWidget {
  final String name;

  const _SeriesFallback({required this.name});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppTokens.boneContainer,
      padding: const EdgeInsets.all(AppTokens.space16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.collections_bookmark_rounded, size: 36, color: AppTokens.mutedCopy),
          const SizedBox(height: AppTokens.space8),
          Text(
            name,
            maxLines: 3,
            textAlign: TextAlign.center,
            overflow: TextOverflow.ellipsis,
            style: AppTypography.titleSerif(fontSize: 12),
          ),
        ],
      ),
    );
  }
}

void showShelfdSettingsModal(
  BuildContext context, {
  required Future<void> Function() onLogout,
}) {
  showModalBottomSheet(
    context: context,
    backgroundColor: AppTokens.boneBackground,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(AppTokens.radiusLg)),
    ),
    builder: (ctx) => SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(AppTokens.space24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                const Icon(Icons.settings_outlined, size: 24, color: AppTokens.charcoalInk),
                const SizedBox(width: AppTokens.space12),
                Text('Settings', style: AppTypography.titleSerif(fontSize: 20)),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.close_rounded),
                  onPressed: () => Navigator.of(ctx).pop(),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space16),
            const Divider(color: AppTokens.crispBorder, height: 1),
            const SizedBox(height: AppTokens.space16),
            Row(
              children: [
                const Icon(Icons.dns_outlined, size: 20, color: AppTokens.mutedCopy),
                const SizedBox(width: AppTokens.space12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Homelab Server',
                        style: AppTypography.bodySans(fontSize: 14, fontWeight: FontWeight.w600),
                      ),
                      Text(
                        'Connected to Shelfd Homelab Daemon',
                        style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                      ),
                    ],
                  ),
                ),
                Container(
                  width: 8,
                  height: 8,
                  decoration: const BoxDecoration(
                    color: Color(0xFF2B8A3E),
                    shape: BoxShape.circle,
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space16),
            Consumer(
              builder: (context, ref, _) {
                final queueState = ref.watch(queueProvider);
                final status = queueState.status;
                if (status == null) return const SizedBox.shrink();

                final libraryState = ref.watch(libraryProvider);
                final resolvedBook = resolveBookTitle(status.currentBook, libraryState.books);
                final progress = (status.progressPercent / 100.0).clamp(0.0, 1.0);

                return Container(
                  padding: const EdgeInsets.all(AppTokens.space12),
                  decoration: BoxDecoration(
                    color: AppTokens.boneSurface,
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                    border: Border.all(color: AppTokens.crispBorder),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          const Icon(Icons.auto_awesome_rounded, size: 16, color: Color(0xFFD9480F)),
                          const SizedBox(width: AppTokens.space8),
                          Text(
                            'AI Passage Indexing & Vectors',
                            style: AppTypography.bodySans(fontSize: 13, fontWeight: FontWeight.w600),
                          ),
                          const Spacer(),
                          Text(
                            '${status.progressPercent.toStringAsFixed(0)}%',
                            style: AppTypography.bodySans(
                              fontSize: 12,
                              fontWeight: FontWeight.w700,
                              color: const Color(0xFFD9480F),
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
                          backgroundColor: AppTokens.boneContainer,
                          valueColor: const AlwaysStoppedAnimation<Color>(Color(0xFFD9480F)),
                        ),
                      ),
                      const SizedBox(height: AppTokens.space8),
                      Text(
                        '${status.indexedChapters} of ${status.totalChapters} chapters processed (${status.pendingChapters} pending)',
                        style: AppTypography.bodySans(fontSize: 12, color: AppTokens.mutedCopy),
                      ),
                      if (resolvedBook != null && resolvedBook.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          'Now processing: $resolvedBook',
                          style: AppTypography.bodySans(
                            fontSize: 11,
                            fontWeight: FontWeight.w500,
                            color: AppTokens.charcoalInk,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ],
                  ),
                );
              },
            ),
            const SizedBox(height: AppTokens.space24),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: AppTokens.charcoalInk,
                side: const BorderSide(color: AppTokens.crispBorder),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
              icon: const Icon(Icons.lock_reset_outlined, size: 18),
              label: const Text('Change Password & Account Settings'),
              onPressed: () {
                Navigator.of(ctx).pop();
                GoRouter.of(context).go('/settings');
              },
            ),
            const SizedBox(height: AppTokens.space12),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: const Color(0xFFC92A2A),
                side: const BorderSide(color: Color(0xFFFFC9C9)),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
              icon: const Icon(Icons.logout_rounded, size: 18),
              label: const Text('Disconnect / Sign Out'),
              onPressed: () async {
                Navigator.of(ctx).pop();
                await onLogout();
              },
            ),
          ],
        ),
      ),
    ),
  );
}


