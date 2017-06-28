import * as React from "react";
import {ILink, ILinkCollection} from "../../managers";
import {NavigationHelper} from "../../NavigationHelper";

interface ICollapsableNavMenuProps {
    links: ILinkCollection[];
    onClick?: (link: ILink) => void;
}

class CollapsableNavMenu extends React.Component<ICollapsableNavMenuProps, undefined> {

    private topItems: HTMLElement[] = [];

    public render() {
        const children = this.props.links.map((e, i) => {
            return this.renderTopElement(i, e);
        });

        return <ul className="nav nav-list">
            {children}
        </ul>;
    }

    private toggle(index: number) {
        const animations: Array<(() => void)> = [];
        this.topItems.forEach((temp, i) => {
            if (i === index) {
                const el: HTMLElement | null = document.getElementById("course-" + index);
                if (this.collapseIsOpen(temp)) {
                    animations.push(this.closeCollapse(temp));
                    if (el) {
                        el.classList.remove("glyphicon-minus-sign");
                        el.classList.add("glyphicon-plus-sign");
                    }

                } else {
                    animations.push(this.openCollapse(temp));
                    if (el) {
                        el.classList.remove("glyphicon-plus-sign");
                        el.classList.add("glyphicon-minus-sign");
                    }
                }
            } else {
                animations.push(this.closeIfOpen(temp));
            }
        });
        setTimeout(() => {
            animations.forEach((e) => {
                e();
            });
        }, 10);
    }

    private collapseIsOpen(ele: HTMLElement) {
        return ele.classList.contains("in");
    }

    private closeIfOpen(ele: HTMLElement): () => void {
        if (this.collapseIsOpen(ele)) {
            return this.closeCollapse(ele);
        }
        return () => {
            "do nothing";
        };
    }

    private openCollapse(ele: HTMLElement): () => void {
        ele.classList.remove("collapse");
        ele.classList.add("collapsing");
        return () => {
            ele.style.height = ele.scrollHeight + "px";
            setTimeout(() => {
                ele.classList.remove("collapsing");
                ele.classList.add("collapse");
                ele.classList.add("in");
                ele.style.height = null;
            }, 350);
        };
    }

    private closeCollapse(ele: HTMLElement): () => void {
        ele.style.height = ele.clientHeight + "px";
        ele.classList.add("collapsing");
        ele.classList.remove("collapse");
        ele.classList.remove("in");
        return () => {
            ele.style.height = null;
            setTimeout(() => {
                ele.classList.remove("collapsing");
                ele.classList.add("collapse");
                ele.style.height = null;
            }, 350);
        };
    }

    private handleClick(e: React.MouseEvent<HTMLAnchorElement>, link: ILink) {
        NavigationHelper.handleClick(e, () => {
            if (this.props.onClick) {
                this.props.onClick(link);
            }
        });
    }

    private renderChilds(index: number, link: ILink): JSX.Element {
        const isActive = link.active ? "active" : "";
        return <li key={index} className={isActive}>
            <a onClick={(e) => this.handleClick(e, link)}
               href={"/" + link.uri}>{link.name}</a>
        </li>;
    }

    private renderTopElement(index: number, links: ILinkCollection): JSX.Element {
        const isActive = links.item.active ? "active" : "";
        const subClass = "nav nav-sub collapse " + (links.item.active ? "in" : "");
        let children: JSX.Element[] = [];
        if (links.children) {
            children = links.children.map((e, i) => {
                return this.renderChilds(i, e);
            });
        }
        return <li key={index} className={isActive}>
            <a
                onClick={(e) => {
                    this.toggle(index);
                    this.handleClick(e, links.item);
                }}
                href={"/" + links.item.uri}>
                <span className="glyphicon glyphicon-plus-sign" id={"course-" + index}></span>
                {links.item.name}
            </a>
            <ul ref={(ele) => {
                if (ele) {
                    this.topItems[index] = ele;
                }
            }}
                className={subClass}>
                {children}
            </ul>
        </li>;
    }
}

export {CollapsableNavMenu};
